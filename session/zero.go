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
	"database/sql"
	"reflect"
	"time"
)

// isNotZero reports whether value is a non-zero Go value.
// It handles common SQL-related types explicitly for performance,
// falling back to reflect for everything else.
func isNotZero(value any) bool {
	if value == nil {
		return false
	}
	switch t := value.(type) {
	case string:
		return t != ""
	case int:
		return t != 0
	case int8:
		return t != 0
	case int16:
		return t != 0
	case int32:
		return t != 0
	case int64:
		return t != 0
	case float32:
		return t != 0
	case float64:
		return t != 0
	case bool:
		return t
	case time.Time:
		return !t.IsZero()
	case sql.NullString:
		return t.Valid
	case sql.NullBool:
		return t.Valid
	case sql.NullInt64:
		return t.Valid
	case sql.NullFloat64:
		return t.Valid
	case sql.NullInt32:
		return t.Valid
	case sql.NullInt16:
		return t.Valid
	case sql.NullTime:
		return t.Valid
	default:
		return !reflect.ValueOf(value).IsZero()
	}
}
