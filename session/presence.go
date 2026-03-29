// Copyright 2025 Dennis Ge
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package session

import (
	"bytes"
	"encoding/json"
	"reflect"
)

// Optional carries both a value and whether it was explicitly provided.
//
// It is useful with Selective methods when zero values such as 0, false, "",
// or nil are still meaningful and should not be treated as "missing".
type Optional[T any] struct {
	Value   T
	Present bool
	Null    bool
}

// Present marks value as explicitly provided for Selective methods.
func Present[T any](value T) Optional[T] {
	return Optional[T]{
		Value:   value,
		Present: true,
		Null:    isNilLikeValue(value),
	}
}

// Absent marks a Selective value as not provided.
func Absent[T any]() Optional[T] {
	var zero T
	return Optional[T]{
		Value:   zero,
		Present: false,
		Null:    false,
	}
}

// Maybe marks value as provided only when present is true.
func Maybe[T any](value T, present bool) Optional[T] {
	return Optional[T]{
		Value:   value,
		Present: present,
		Null:    present && isNilLikeValue(value),
	}
}

// Get returns the wrapped value plus whether it was explicitly provided.
func (o Optional[T]) Get() (T, bool) {
	return o.Value, o.Present
}

// IsPresent reports whether the wrapped value was explicitly provided.
func (o Optional[T]) IsPresent() bool {
	return o.Present
}

// IsNull reports whether the value was explicitly provided as null.
func (o Optional[T]) IsNull() bool {
	return o.Present && o.Null
}

// HasValue reports whether the value was explicitly provided and is not null.
func (o Optional[T]) HasValue() bool {
	return o.Present && !o.Null
}

// IsZero reports whether the optional was never provided.
//
// It enables JSON tags such as `json:",omitzero"` to omit absent optionals
// while still encoding explicit null values as JSON null.
func (o Optional[T]) IsZero() bool {
	return !o.Present
}

func (o Optional[T]) selectivePresence() (any, bool, bool) {
	if o.Null {
		return nil, o.Present, true
	}
	return o.Value, o.Present, false
}

type selectivePresence interface {
	selectivePresence() (any, bool, bool)
}

// UnmarshalJSON marks the field as present whenever it appears in input JSON.
// Missing fields remain zero-valued with Present=false.
func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	o.Present = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		var zero T
		o.Value = zero
		o.Null = true
		return nil
	}

	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	o.Value = value
	o.Null = isNilLikeValue(value)
	return nil
}

// MarshalJSON encodes present values directly and encodes null optionals as JSON null.
//
// Use `json:",omitzero"` on the containing struct field when you want absent
// optionals to be omitted from output entirely.
func (o Optional[T]) MarshalJSON() ([]byte, error) {
	if !o.Present || o.Null {
		return []byte("null"), nil
	}
	return json.Marshal(o.Value)
}

func selectiveValueState(value any) (any, bool) {
	if actual, present, _, wrapped := unwrapSelectivePresence(value); wrapped {
		return actual, present
	}
	return value, isNotZero(value)
}

func unwrapSelectivePresence(value any) (any, bool, bool, bool) {
	if value == nil {
		return nil, false, false, false
	}

	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Pointer && rv.IsNil() {
		if rv.Type().Implements(reflect.TypeFor[selectivePresence]()) {
			return nil, false, false, true
		}
		return nil, false, false, false
	}

	if carrier, ok := value.(selectivePresence); ok {
		actual, present, null := carrier.selectivePresence()
		return actual, present, null, true
	}

	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, false, false, false
		}
		rv = rv.Elem()
		if rv.CanInterface() {
			if carrier, ok := rv.Interface().(selectivePresence); ok {
				actual, present, null := carrier.selectivePresence()
				return actual, present, null, true
			}
		}
	}

	return value, false, false, false
}

func isNilLikeValue(value any) bool {
	if value == nil {
		return true
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
