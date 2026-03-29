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
	"reflect"
	"strings"
)

func (s *baseSession) bindNamedArgs(sqlText string, arg any, caller string) error {
	for _, ph := range getPlaceholders(sqlText) {
		name := placeholderName(ph)
		value, ok, err := lookupNamedArgValue(arg, name)
		if err != nil {
			return sessionWrapError(caller, err)
		}
		if !ok {
			return sessionErrorf("%s: missing named value for placeholder %s", caller, ph)
		}
		if actual, present, _, wrapped := unwrapSelectivePresence(value); wrapped {
			if !present {
				return sessionErrorf("%s: missing named value for placeholder %s", caller, ph)
			}
			value = actual
		}
		if err := s.bindPlaceholder(ph, value); err != nil {
			return sessionWrapError(caller, err)
		}
	}
	return nil
}

func lookupNamedArgValue(arg any, name string) (any, bool, error) {
	if arg == nil {
		return nil, false, sessionErrorf("named arg source cannot be nil")
	}

	value := reflect.ValueOf(arg)
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil, false, sessionErrorf("named arg source cannot be nil")
		}
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.Map:
		if value.Type().Key().Kind() != reflect.String {
			return nil, false, sessionErrorf("named arg source map must use string keys")
		}
		mappedValue, ok := lookupNamedMapValue(value, name)
		return mappedValue, ok, nil
	case reflect.Struct:
		index, ok := buildFieldIndex(value.Type())[strings.ToLower(name)]
		if !ok {
			return nil, false, nil
		}
		field, ok := fieldByIndexValue(value, index)
		if !ok {
			return nil, false, nil
		}
		return reflectValueInterface(field), true, nil
	default:
		return nil, false, sessionErrorf("named arg source must be a struct or map with string keys, got %T", arg)
	}
}

func supportsNamedArgSource(arg any) bool {
	if actual, present, _, wrapped := unwrapSelectivePresence(arg); wrapped {
		if !present {
			return false
		}
		arg = actual
	}
	if arg == nil {
		return false
	}

	typ := reflect.TypeOf(arg)
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	switch typ.Kind() {
	case reflect.Struct:
		return true
	case reflect.Map:
		return typ.Key().Kind() == reflect.String
	default:
		return false
	}
}

func lookupNamedMapValue(value reflect.Value, name string) (any, bool) {
	lowerName := strings.ToLower(name)
	for _, key := range value.MapKeys() {
		keyText := key.String()
		if keyText == name || strings.ToLower(keyText) == lowerName {
			return reflectValueInterface(value.MapIndex(key)), true
		}
	}
	return nil, false
}

func fieldByIndexValue(value reflect.Value, index []int) (reflect.Value, bool) {
	current := value
	for _, part := range index {
		if current.Kind() == reflect.Pointer {
			if current.IsNil() {
				return reflect.Value{}, false
			}
			current = current.Elem()
		}
		if current.Kind() != reflect.Struct {
			return reflect.Value{}, false
		}
		current = current.Field(part)
	}
	return current, true
}

func reflectValueInterface(value reflect.Value) any {
	if !value.IsValid() {
		return nil
	}
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		return reflectValueInterface(value.Elem())
	}
	return value.Interface()
}
