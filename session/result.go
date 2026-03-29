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
	"errors"
	"fmt"
)

var (
	// ErrLastInsertIDUnavailable reports that the current statement did not expose a generated ID.
	ErrLastInsertIDUnavailable = errors.New("session: last insert id is unavailable for this statement")
	// ErrLastInsertIDUnsupported reports that the backend/database does not support fetching a generated ID via Exec.
	ErrLastInsertIDUnsupported = errors.New("session: last insert id is unsupported by this provider/database; use Returning(...).Scan(...) when available")
)

// ExecutionResult describes the outcome of an Exec-style statement.
type ExecutionResult struct {
	RowsAffected    int64
	LastInsertID    int64
	HasLastInsertID bool
	lastInsertIDErr error
}

// InsertID returns the generated ID when the provider/database exposes one.
func (r ExecutionResult) InsertID() (int64, error) {
	if r.HasLastInsertID {
		return r.LastInsertID, nil
	}
	if r.lastInsertIDErr != nil {
		return 0, r.lastInsertIDErr
	}
	return 0, ErrLastInsertIDUnavailable
}

// InsertIDs derives a contiguous sequence of IDs from the first generated ID and rows affected.
//
// This is intended for auto-increment databases that report the first inserted ID for the statement,
// such as MySQL's LastInsertId behavior. It is not appropriate for databases that require RETURNING
// to expose generated IDs.
func (r ExecutionResult) InsertIDs() ([]int64, error) {
	return r.InsertIDsWithStep(1)
}

// InsertIDsWithStep derives a contiguous sequence of IDs from the first generated ID and rows affected,
// using the provided increment step.
func (r ExecutionResult) InsertIDsWithStep(step int64) ([]int64, error) {
	if step <= 0 {
		return nil, fmt.Errorf("session: insert id step must be > 0, got %d", step)
	}
	firstID, err := r.InsertID()
	if err != nil {
		return nil, err
	}
	if r.RowsAffected <= 0 {
		return []int64{}, nil
	}
	ids := make([]int64, 0, r.RowsAffected)
	for i := int64(0); i < r.RowsAffected; i++ {
		ids = append(ids, firstID+(i*step))
	}
	return ids, nil
}

func withLastInsertID(result ExecutionResult, id int64) ExecutionResult {
	result.LastInsertID = id
	result.HasLastInsertID = true
	result.lastInsertIDErr = nil
	return result
}

func withLastInsertIDError(result ExecutionResult, err error) ExecutionResult {
	result.HasLastInsertID = false
	result.lastInsertIDErr = err
	return result
}
