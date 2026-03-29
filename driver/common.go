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

package driver

import (
	"database/sql"

	"gorm.io/gorm"
)

// ConfigureSQLPool applies connection pool settings to a standard library sql.DB.
func ConfigureSQLPool(db *sql.DB, cfg *Config) {
	if db == nil || cfg == nil {
		return
	}
	sqlDB := db
	sqlDB.SetMaxIdleConns(cfg.MaxIdle)
	sqlDB.SetMaxOpenConns(cfg.MaxOpen)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
}

// ConfigurePool applies connection pool settings to a GORM DB.
func ConfigurePool(db *gorm.DB, cfg *Config) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	ConfigureSQLPool(sqlDB, cfg)
	return nil
}

// Close closes the underlying sql.DB of a GORM DB.
func Close(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// CloseSQL closes a standard library sql.DB.
func CloseSQL(db *sql.DB) error {
	if db == nil {
		return nil
	}
	return db.Close()
}
