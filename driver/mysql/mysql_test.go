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

package mysql

import (
	"errors"
	"testing"

	sqldriver "github.com/dennisge/sqlcraft/driver"
	"github.com/dennisge/sqlcraft/session"
)

func TestNewSessionReturnsStdSession(t *testing.T) {
	got := NewSession(nil)
	if _, ok := got.(*session.StdSession); !ok {
		t.Fatalf("NewSession() type = %T, want *session.StdSession", got)
	}
}

func TestNewGormSessionReturnsGormSession(t *testing.T) {
	got := NewGormSession(nil)
	if _, ok := got.(*session.GormSession); !ok {
		t.Fatalf("NewGormSession() type = %T, want *session.GormSession", got)
	}
}

func TestOpenSessionRejectsMissingDSN(t *testing.T) {
	_, err := OpenSession(nil)
	if !errors.Is(err, sqldriver.ErrDSNRequired) {
		t.Fatalf("OpenSession() error = %v, want %v", err, sqldriver.ErrDSNRequired)
	}
}

func TestOpenGormSessionRejectsMissingDSN(t *testing.T) {
	_, err := OpenGormSession(nil)
	if !errors.Is(err, sqldriver.ErrDSNRequired) {
		t.Fatalf("OpenGormSession() error = %v, want %v", err, sqldriver.ErrDSNRequired)
	}
}
