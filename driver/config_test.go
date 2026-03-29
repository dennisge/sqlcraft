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
	"errors"
	"testing"
)

func TestNormalizeConfigAppliesDefaultsWithoutMutatingInput(t *testing.T) {
	original := &Config{DSN: "postgres://example"}

	normalized := NormalizeConfig(original)

	if normalized == original {
		t.Fatal("NormalizeConfig() should return a copy")
	}
	if normalized.MaxOpen != 25 || normalized.MaxIdle != 10 {
		t.Fatalf("NormalizeConfig() = %#v, want default pool settings", normalized)
	}
	if original.MaxOpen != 0 || original.MaxIdle != 0 {
		t.Fatalf("original config was mutated: %#v", original)
	}
}

func TestConfigValidateRequiresDSN(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
	}{
		{name: "nil config", cfg: nil},
		{name: "empty dsn", cfg: &Config{}},
		{name: "blank dsn", cfg: &Config{DSN: "   "}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.Validate(); !errors.Is(err, ErrDSNRequired) {
				t.Fatalf("Validate() error = %v, want %v", err, ErrDSNRequired)
			}
		})
	}
}
