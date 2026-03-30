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
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// Dialect controls how the database/sql provider renders placeholders.
type Dialect string

const (
	// DialectMySQL renders placeholders as ?.
	DialectMySQL Dialect = "mysql"
	// DialectPostgres renders placeholders as $1, $2, ...
	DialectPostgres Dialect = "postgres"
)

func (d Dialect) placeholder(index int) (string, error) {
	switch d {
	case DialectMySQL:
		return "?", nil
	case DialectPostgres:
		return fmt.Sprintf("$%d", index), nil
	default:
		return "", sessionErrorf("unsupported dialect %q", d)
	}
}

func newSessionForBackend(backend sessionBackend) Session {
	base := newBaseSession(backend)
	switch backend.(type) {
	case *gormBackend:
		session := &GormSession{baseSession: base}
		base.self = session
		return session
	case *stdBackend:
		session := &StdSession{baseSession: base}
		base.self = session
		return session
	default:
		base.self = base
		return base
	}
}

func renderSQL(sqlText string, argMap map[string]any, binder func(name string, value any, index int) (string, any, error)) (string, []any, error) {
	if sqlText == "" {
		return "", nil, nil
	}

	var (
		args []any
		sb   strings.Builder
	)

	for i := 0; i < len(sqlText); {
		if i+1 >= len(sqlText) || (sqlText[i] != '#' && sqlText[i] != '$') || sqlText[i+1] != '{' {
			sb.WriteByte(sqlText[i])
			i++
			continue
		}

		end := strings.IndexByte(sqlText[i+2:], '}')
		if end < 0 {
			sb.WriteByte(sqlText[i])
			i++
			continue
		}
		end += i + 2

		ph := sqlText[i : end+1]
		value, exists := argMap[ph]
		if !exists {
			return "", nil, sessionErrorf("missing value for placeholder %s", ph)
		}

		if strings.HasPrefix(ph, "${") {
			sb.WriteString(fmt.Sprintf("%v", value))
		} else {
			name := placeholderName(ph)
			token, arg, err := binder(name, value, len(args)+1)
			if err != nil {
				return "", nil, err
			}
			sb.WriteString(token)
			args = append(args, arg)
		}

		i = end + 1
	}

	return sb.String(), args, nil
}

func firstTxOptions(opts []*sql.TxOptions) *sql.TxOptions {
	for _, opt := range opts {
		if opt != nil {
			return opt
		}
	}
	return nil
}

// New creates a new [Session] backed by GORM.
func New(db *gorm.DB) Session {
	return NewGorm(db)
}

// NewGorm creates a new [Session] backed by GORM.
func NewGorm(db *gorm.DB) Session {
	return newSessionForBackend(&gormBackend{db: db})
}

// NewStd creates a new [Session] backed by database/sql with the given dialect.
func NewStd(db *sql.DB, dialect Dialect) Session {
	return newSessionForBackend(&stdBackend{db: db, dialect: dialect})
}

// NewStdTx creates a new [Session] backed by an existing database/sql transaction.
func NewStdTx(tx *sql.Tx, dialect Dialect) Session {
	return newSessionForBackend(&stdBackend{tx: tx, dialect: dialect})
}

// NewStdMySQL creates a new database/sql-backed [Session] using MySQL placeholders.
func NewStdMySQL(db *sql.DB) Session {
	return NewStd(db, DialectMySQL)
}

// NewStdPostgres creates a new database/sql-backed [Session] using PostgreSQL placeholders.
func NewStdPostgres(db *sql.DB) Session {
	return NewStd(db, DialectPostgres)
}
