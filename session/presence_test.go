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
	"encoding/json"
	"reflect"
	"testing"
)

func TestOptionalJSONUnmarshalMissingFieldKeepsAbsent(t *testing.T) {
	type request struct {
		Status Optional[int] `json:"status"`
	}

	var req request
	if err := json.Unmarshal([]byte(`{}`), &req); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if req.Status.IsPresent() {
		t.Fatalf("Status.IsPresent() = true, want false")
	}
	if req.Status.IsNull() {
		t.Fatalf("Status.IsNull() = true, want false")
	}
}

func TestOptionalJSONUnmarshalZeroValueMarksPresent(t *testing.T) {
	type request struct {
		Status Optional[int] `json:"status"`
	}

	var req request
	if err := json.Unmarshal([]byte(`{"status":0}`), &req); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if !req.Status.IsPresent() {
		t.Fatalf("Status.IsPresent() = false, want true")
	}
	if req.Status.IsNull() {
		t.Fatalf("Status.IsNull() = true, want false")
	}
	if req.Status.Value != 0 {
		t.Fatalf("Status.Value = %d, want 0", req.Status.Value)
	}
}

func TestOptionalJSONUnmarshalNullMarksPresentAndNull(t *testing.T) {
	type request struct {
		DeletedAt Optional[*string] `json:"deleted_at"`
	}

	var req request
	if err := json.Unmarshal([]byte(`{"deleted_at":null}`), &req); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if !req.DeletedAt.IsPresent() {
		t.Fatalf("DeletedAt.IsPresent() = false, want true")
	}
	if !req.DeletedAt.IsNull() {
		t.Fatalf("DeletedAt.IsNull() = false, want true")
	}
	if req.DeletedAt.Value != nil {
		t.Fatalf("DeletedAt.Value = %#v, want nil", req.DeletedAt.Value)
	}
}

func TestOptionalJSONMarshalKeepsPresenceSemantics(t *testing.T) {
	type request struct {
		Status    Optional[int]     `json:"status"`
		DeletedAt Optional[*string] `json:"deleted_at"`
	}

	payload, err := json.Marshal(request{
		Status:    Present(0),
		DeletedAt: Present[*string](nil),
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("json.Unmarshal(roundtrip) error = %v", err)
	}
	if !reflect.DeepEqual(got["status"], float64(0)) {
		t.Fatalf("status = %#v, want 0", got["status"])
	}
	if value, exists := got["deleted_at"]; !exists || value != nil {
		t.Fatalf("deleted_at = %#v, want null", got["deleted_at"])
	}
}

func TestOptionalJSONMarshalOmitsAbsentWithOmitZero(t *testing.T) {
	type response struct {
		Status    Optional[int]     `json:"status,omitzero"`
		DeletedAt Optional[*string] `json:"deleted_at,omitzero"`
	}

	payload, err := json.Marshal(response{
		Status:    Absent[int](),
		DeletedAt: Present[*string](nil),
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("json.Unmarshal(roundtrip) error = %v", err)
	}
	if _, exists := got["status"]; exists {
		t.Fatalf("status exists in payload, want omitted: %#v", got)
	}
	if value, exists := got["deleted_at"]; !exists || value != nil {
		t.Fatalf("deleted_at = %#v, want null", got["deleted_at"])
	}
}

func TestWhereSelectiveSupportsJSONMappedOptional(t *testing.T) {
	type request struct {
		Status Optional[int] `json:"status"`
	}

	var req request
	if err := json.Unmarshal([]byte(`{"status":0}`), &req); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	db, _ := openTestDB(t)
	defer db.Close()

	session := NewStdMySQL(db).
		Select("id").
		From("users").
		WhereSelective("status = #{status}", req.Status)

	snap := session.(interface{ snapshot() sessionSnapshot }).snapshot()
	if snap.sqlText != "SELECT id\nFROM users\nWHERE (status = #{status})" {
		t.Fatalf("sqlText = %q", snap.sqlText)
	}
	if !reflect.DeepEqual(snap.argMap, map[string]any{"#{status}": 0}) {
		t.Fatalf("argMap = %#v", snap.argMap)
	}
}

func TestAppendRawSelectiveSupportsJSONMappedNullGate(t *testing.T) {
	type request struct {
		DeletedAt Optional[*string] `json:"deleted_at"`
	}

	var req request
	if err := json.Unmarshal([]byte(`{"deleted_at":null}`), &req); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	db, _ := openTestDB(t)
	defer db.Close()

	session := NewStdMySQL(db).
		Select("id").
		From("users").
		AppendRawSelective("WHERE deleted_at IS NULL", req.DeletedAt.IsNull())

	snap := session.(interface{ snapshot() sessionSnapshot }).snapshot()
	if snap.sqlText != "SELECT id\nFROM users WHERE deleted_at IS NULL" {
		t.Fatalf("sqlText = %q", snap.sqlText)
	}
}
