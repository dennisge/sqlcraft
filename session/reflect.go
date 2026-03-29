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
	"fmt"
	"reflect"
)

// SelectiveFields returns the exported field names of a struct (or pointer to struct)
// whose values are not zero. This is useful for building INSERT/UPDATE statements
// that only include non-zero fields.
//
// Panics if value is not a struct or pointer to struct.
func SelectiveFields(value any) []string {
	if value == nil {
		panic("session: SelectiveFields expects struct or *struct, got <nil>")
	}

	columns := make([]string, 0)
	switch reflect.TypeOf(value).Kind() {
	case reflect.Pointer:
		elem := reflect.ValueOf(value).Elem()
		return collectStructFields(elem, columns)
	case reflect.Struct:
		return collectStructFields(reflect.ValueOf(value), columns)
	default:
		panic(fmt.Sprintf("session: SelectiveFields expects struct or *struct, got %T", value))
	}
}

func collectStructFields(value reflect.Value, columns []string) []string {
	typ := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		val := value.Field(i)
		if field.Anonymous {
			if val.Kind() == reflect.Pointer && val.IsValid() && val.Elem().IsValid() && val.Elem().Kind() == reflect.Struct {
				columns = append(columns, collectStructFields(val.Elem(), nil)...)
			} else if val.Kind() == reflect.Struct {
				columns = append(columns, collectStructFields(val, nil)...)
			}
		} else {
			if val.IsValid() && !val.IsZero() {
				columns = append(columns, field.Name)
			}
		}
	}
	return columns
}
