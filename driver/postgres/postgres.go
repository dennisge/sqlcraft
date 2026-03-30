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

// Package postgres provides a PostgreSQL driver initializer for sqlcraft.
package postgres

import (
	"database/sql"

	"github.com/dennisge/sqlcraft/session"
	_ "github.com/jackc/pgx/v5/stdlib"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/dennisge/sqlcraft/driver"
)

// Open creates a new GORM DB connection for PostgreSQL with the given config.
// Optional gorm.Config can be passed to customize GORM behavior.
func Open(cfg *driver.Config, gormCfg ...*gorm.Config) (*gorm.DB, error) {
	return OpenGorm(cfg, gormCfg...)
}

// OpenGorm creates a new GORM DB connection for PostgreSQL with the given config.
// Optional gorm.Config can be passed to customize GORM behavior.
func OpenGorm(cfg *driver.Config, gormCfg ...*gorm.Config) (*gorm.DB, error) {
	cfg = driver.NormalizeConfig(cfg)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	var gc *gorm.Config
	if len(gormCfg) > 0 && gormCfg[0] != nil {
		gc = gormCfg[0]
	} else {
		gc = &gorm.Config{}
	}
	db, err := gorm.Open(gormpostgres.New(gormpostgres.Config{DSN: cfg.DSN}), gc)
	if err != nil {
		return nil, err
	}
	if err := driver.ConfigurePool(db, cfg); err != nil {
		return nil, err
	}
	return db, nil
}

// OpenStd creates a new database/sql DB connection for PostgreSQL with the given config.
func OpenStd(cfg *driver.Config) (*sql.DB, error) {
	cfg = driver.NormalizeConfig(cfg)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	db, err := sql.Open("pgx", cfg.DSN)
	if err != nil {
		return nil, err
	}
	driver.ConfigureSQLPool(db, cfg)
	return db, nil
}

// NewSession binds an existing database/sql DB to a PostgreSQL-flavored sqlcraft session.
func NewSession(db *sql.DB) session.Session {
	return session.NewStdPostgres(db)
}

// NewTxSession binds an existing database/sql transaction to a PostgreSQL-flavored sqlcraft session.
func NewTxSession(tx *sql.Tx) session.Session {
	return session.NewStdTx(tx, session.DialectPostgres)
}

// NewGormSession binds an existing GORM DB to a sqlcraft session.
func NewGormSession(db *gorm.DB) session.Session {
	return session.NewGorm(db)
}

// OpenSession opens a native database/sql connection and returns a PostgreSQL-flavored sqlcraft session.
func OpenSession(cfg *driver.Config) (session.Session, error) {
	db, err := OpenStd(cfg)
	if err != nil {
		return nil, err
	}
	return NewSession(db), nil
}

// OpenGormSession opens a GORM connection and returns a sqlcraft session bound to it.
func OpenGormSession(cfg *driver.Config, gormCfg ...*gorm.Config) (session.Session, error) {
	db, err := OpenGorm(cfg, gormCfg...)
	if err != nil {
		return nil, err
	}
	return NewGormSession(db), nil
}
