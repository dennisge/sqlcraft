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

// Package driver provides helpers for initializing GORM database connections.
package driver

import (
	"errors"
	"strings"
	"time"
)

var (
	// ErrDSNRequired reports that a database DSN was not provided.
	ErrDSNRequired = errors.New("driver: dsn is required")
)

// Config holds database connection pool settings.
type Config struct {
	// DSN is the data source name.
	DSN string `json:"dsn" yaml:"dsn"`

	// MaxOpen is the maximum number of open connections. Default: 25.
	MaxOpen int `json:"maxOpen" yaml:"maxOpen"`

	// MaxIdle is the maximum number of idle connections. Default: 10.
	MaxIdle int `json:"maxIdle" yaml:"maxIdle"`

	// ConnMaxLifetime is the maximum lifetime of a connection. Default: 5m.
	ConnMaxLifetime time.Duration `json:"connMaxLifetime" yaml:"connMaxLifetime"`

	// ConnMaxIdleTime is the maximum idle time of a connection. Default: 3m.
	ConnMaxIdleTime time.Duration `json:"connMaxIdleTime" yaml:"connMaxIdleTime"`
}

// SetDefaults fills zero-valued fields with sensible defaults.
func (c *Config) SetDefaults() {
	if c == nil {
		return
	}
	if c.MaxOpen == 0 {
		c.MaxOpen = 25
	}
	if c.MaxIdle == 0 {
		c.MaxIdle = 10
	}
	if c.ConnMaxLifetime == 0 {
		c.ConnMaxLifetime = 5 * time.Minute
	}
	if c.ConnMaxIdleTime == 0 {
		c.ConnMaxIdleTime = 3 * time.Minute
	}
}

// Validate ensures the config contains the required database settings.
func (c *Config) Validate() error {
	if c == nil || strings.TrimSpace(c.DSN) == "" {
		return ErrDSNRequired
	}
	return nil
}

// NormalizeConfig returns a copy of cfg with defaults applied.
// A nil cfg is treated as an empty config.
func NormalizeConfig(cfg *Config) *Config {
	if cfg == nil {
		cfg = &Config{}
	}
	normalized := *cfg
	normalized.SetDefaults()
	return &normalized
}
