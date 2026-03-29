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
	cfg.SetDefaults()
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
	cfg.SetDefaults()
	db, err := sql.Open("pgx", cfg.DSN)
	if err != nil {
		return nil, err
	}
	driver.ConfigureSQLPool(db, cfg)
	return db, nil
}
