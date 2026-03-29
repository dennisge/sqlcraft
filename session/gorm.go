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
	"strings"

	"gorm.io/gorm"
)

// GormSession is a [Session] implementation backed by GORM.
type GormSession struct {
	*baseSession
}

type gormBackend struct {
	db *gorm.DB
}

func (b *gormBackend) scan(sqlText string, argMap map[string]any, debug bool, dest any) error {
	query, args, err := renderSQL(sqlText, argMap, func(name string, value any, _ int) (string, any, error) {
		return "@" + name, sql.Named(name, value), nil
	})
	if err != nil {
		return err
	}

	db := b.db
	if debug {
		db = db.Debug()
	}
	return db.Raw(query, args...).Scan(dest).Error
}

func (b *gormBackend) execResult(sqlText string, argMap map[string]any, debug bool) (ExecutionResult, error) {
	query, args, err := renderSQL(sqlText, argMap, func(name string, value any, _ int) (string, any, error) {
		return "@" + name, sql.Named(name, value), nil
	})
	if err != nil {
		return ExecutionResult{}, err
	}

	result := ExecutionResult{}
	err = b.db.Connection(func(tx *gorm.DB) error {
		db := tx
		if debug {
			db = db.Debug()
		}
		execResult := db.Exec(query, args...)
		if execResult.Error != nil {
			return execResult.Error
		}
		result.RowsAffected = execResult.RowsAffected
		return b.fillLastInsertID(tx, query, &result)
	})
	if err != nil {
		return ExecutionResult{}, err
	}
	return result, nil
}

func (b *gormBackend) transaction(fc func(sessionBackend) error, opts ...*sql.TxOptions) error {
	return b.db.Transaction(func(tx *gorm.DB) error {
		return fc(&gormBackend{db: tx})
	}, opts...)
}

func (b *gormBackend) renew() sessionBackend {
	return &gormBackend{db: b.db}
}

func (b *gormBackend) fillLastInsertID(tx *gorm.DB, query string, result *ExecutionResult) error {
	if !isInsertStatement(query) {
		*result = withLastInsertIDError(*result, ErrLastInsertIDUnavailable)
		return nil
	}

	switch tx.Dialector.Name() {
	case "mysql":
		var id int64
		if err := tx.Raw("SELECT LAST_INSERT_ID()").Scan(&id).Error; err != nil {
			return err
		}
		*result = withLastInsertID(*result, id)
	default:
		*result = withLastInsertIDError(*result, ErrLastInsertIDUnsupported)
	}
	return nil
}

// Create inserts model into table using GORM's Create. Returns affected rows.
// If columns are specified, only those columns are inserted.
func (s *GormSession) Create(table string, model any, columns ...string) (int64, error) {
	if s.err != nil {
		err := s.err
		s.Reset()
		return 0, err
	}
	backend, _ := s.backend.(*gormBackend)
	db := backend.db
	if s.debug {
		db = db.Debug()
	}
	s.Reset()
	result := db.Table(table).Select(columns).Create(model)
	return result.RowsAffected, result.Error
}

// InsertSelective inserts model into table using GORM's struct-based insert.
func (s *GormSession) InsertSelective(table string, model any) (int64, error) {
	if s.err != nil {
		err := s.err
		s.Reset()
		return 0, err
	}
	backend, _ := s.backend.(*gormBackend)
	db := backend.db
	if s.debug {
		db = db.Debug()
	}
	s.Reset()
	result := db.Table(table).Create(model)
	return result.RowsAffected, result.Error
}

func isInsertStatement(query string) bool {
	normalized := strings.TrimSpace(strings.ToUpper(query))
	return strings.HasPrefix(normalized, "INSERT ")
}
