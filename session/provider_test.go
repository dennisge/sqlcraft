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
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestStdMySQLExecUsesPositionalArgs(t *testing.T) {
	db, state := openTestDB(t)
	defer db.Close()

	rowsAffected, err := NewStdMySQL(db).
		Update("users").
		Set("status", 2).
		Where("id = #{id}", 7).
		Exec()
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}
	if rowsAffected != state.execRowsAffected {
		t.Fatalf("rowsAffected = %d, want %d", rowsAffected, state.execRowsAffected)
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.execCalls) != 1 {
		t.Fatalf("execCalls = %d, want 1", len(state.execCalls))
	}
	got := state.execCalls[0]
	wantSQL := "UPDATE users\nSET status = ?\nWHERE (id = ?)"
	if got.query != wantSQL {
		t.Fatalf("exec query = %q, want %q", got.query, wantSQL)
	}
	assertIntArgs(t, got.args, 2, 7)
}

func TestStdPostgresScanMapsColumnsToStruct(t *testing.T) {
	db, state := openTestDB(t)
	defer db.Close()

	state.setRows([]string{"id", "user_name", "status"}, [][]driver.Value{
		{int64(1), []byte("alice"), int64(1)},
		{int64(2), []byte("bob"), int64(0)},
	})

	type User struct {
		ID       int64
		UserName string `db:"user_name"`
		Status   int
	}

	var users []User
	err := NewStdPostgres(db).
		Select("id", "user_name", "status").
		From("users").
		Where("status = #{status}", 1).
		Scan(&users)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	wantUsers := []User{
		{ID: 1, UserName: "alice", Status: 1},
		{ID: 2, UserName: "bob", Status: 0},
	}
	if !reflect.DeepEqual(users, wantUsers) {
		t.Fatalf("users = %#v, want %#v", users, wantUsers)
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.queryCalls) != 1 {
		t.Fatalf("queryCalls = %d, want 1", len(state.queryCalls))
	}
	got := state.queryCalls[0]
	wantSQL := "SELECT id, user_name, status\nFROM users\nWHERE (status = $1)"
	if got.query != wantSQL {
		t.Fatalf("query = %q, want %q", got.query, wantSQL)
	}
	assertIntArgs(t, got.args, 1)
}

func TestStdExecResultExposesLastInsertID(t *testing.T) {
	db, state := openTestDB(t)
	defer db.Close()

	state.mu.Lock()
	state.execRowsAffected = 3
	state.execLastInsertID = 41
	state.supportLastInsertID = true
	state.mu.Unlock()

	result, err := NewStdMySQL(db).
		InsertInto("users").
		IntoColumns("name").
		IntoMultiValues([][]any{{"alice"}, {"bob"}, {"charlie"}}).
		ExecResult()
	if err != nil {
		t.Fatalf("ExecResult() error = %v", err)
	}
	if result.RowsAffected != 3 {
		t.Fatalf("RowsAffected = %d, want 3", result.RowsAffected)
	}
	insertID, err := result.InsertID()
	if err != nil {
		t.Fatalf("InsertID() error = %v", err)
	}
	if insertID != 41 {
		t.Fatalf("InsertID = %d, want 41", insertID)
	}
	ids, err := result.InsertIDs()
	if err != nil {
		t.Fatalf("InsertIDs() error = %v", err)
	}
	if !reflect.DeepEqual(ids, []int64{41, 42, 43}) {
		t.Fatalf("InsertIDs = %v, want [41 42 43]", ids)
	}
}

func TestStdExecResultReportsUnsupportedLastInsertID(t *testing.T) {
	db, state := openTestDB(t)
	defer db.Close()

	state.mu.Lock()
	state.supportLastInsertID = false
	state.mu.Unlock()

	result, err := NewStdPostgres(db).
		InsertInto("users").
		Values("name", "alice").
		ExecResult()
	if err != nil {
		t.Fatalf("ExecResult() error = %v", err)
	}
	if _, err := result.InsertID(); !errors.Is(err, ErrLastInsertIDUnsupported) {
		t.Fatalf("InsertID() error = %v, want %v", err, ErrLastInsertIDUnsupported)
	}
}

func TestReturningClauseCanBeScanned(t *testing.T) {
	db, state := openTestDB(t)
	defer db.Close()

	state.setRows([]string{"id"}, [][]driver.Value{
		{int64(9)},
		{int64(10)},
	})

	var rows []struct {
		ID int64 `db:"id"`
	}
	err := NewStdPostgres(db).
		InsertInto("users").
		IntoColumns("name").
		IntoMultiValues([][]any{{"alice"}, {"bob"}}).
		Returning("id").
		Scan(&rows)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if !reflect.DeepEqual(rows, []struct {
		ID int64 `db:"id"`
	}{{ID: 9}, {ID: 10}}) {
		t.Fatalf("rows = %#v", rows)
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	got := state.queryCalls[len(state.queryCalls)-1]
	wantSQL := "INSERT INTO users\n (name)\nVALUES ($1)\n, ($2)\nRETURNING id"
	if got.query != wantSQL {
		t.Fatalf("query = %q, want %q", got.query, wantSQL)
	}
}

func TestAppendRebindsConflictingPlaceholders(t *testing.T) {
	db, _ := openTestDB(t)
	defer db.Close()

	left := NewStdMySQL(db).Where("id = #{id}", 1)
	right := NewStdMySQL(db).AppendRaw("OR owner_id = #{id}", 2)

	session := NewStdMySQL(db).Select("id").From("users").Append(left).Append(right)
	snap := session.(interface{ snapshot() sessionSnapshot }).snapshot()
	sqlText := snap.sqlText
	argMap := snap.argMap

	if sqlText != "SELECT id\nFROM users WHERE (id = #{id}) OR owner_id = #{id_0}" {
		t.Fatalf("sqlText = %q", sqlText)
	}
	if !reflect.DeepEqual(argMap["#{id}"], 1) {
		t.Fatalf("argMap[#{id}] = %#v, want 1", argMap["#{id}"])
	}
	if !reflect.DeepEqual(argMap["#{id_0}"], 2) {
		t.Fatalf("argMap[#{id_0}] = %#v, want 2", argMap["#{id_0}"])
	}
}

func TestAppendRawSelectiveSkipsZeroCondition(t *testing.T) {
	db, _ := openTestDB(t)
	defer db.Close()

	session := NewStdMySQL(db).
		Select("id").
		From("users").
		AppendRawSelective("FOR UPDATE", false)

	snap := session.(interface{ snapshot() sessionSnapshot }).snapshot()
	if snap.sqlText != "SELECT id\nFROM users" {
		t.Fatalf("sqlText = %q, want %q", snap.sqlText, "SELECT id\nFROM users")
	}
}

func TestAppendRawSelectiveAppendsWhenConditionMatches(t *testing.T) {
	db, _ := openTestDB(t)
	defer db.Close()

	session := NewStdMySQL(db).
		Select("id").
		From("users").
		AppendRawSelective("FOR UPDATE", true)

	snap := session.(interface{ snapshot() sessionSnapshot }).snapshot()
	if snap.sqlText != "SELECT id\nFROM users FOR UPDATE" {
		t.Fatalf("sqlText = %q, want %q", snap.sqlText, "SELECT id\nFROM users FOR UPDATE")
	}
}

func TestAppendRawSelectiveSupportsPresentFalseGate(t *testing.T) {
	db, _ := openTestDB(t)
	defer db.Close()

	session := NewStdMySQL(db).
		Select("id").
		From("users").
		AppendRawSelective("FOR UPDATE", Present(false))

	snap := session.(interface{ snapshot() sessionSnapshot }).snapshot()
	if snap.sqlText != "SELECT id\nFROM users FOR UPDATE" {
		t.Fatalf("sqlText = %q, want %q", snap.sqlText, "SELECT id\nFROM users FOR UPDATE")
	}
}

func TestAppendRawSelectiveBindsNamedFieldsWhenAllPresent(t *testing.T) {
	db, _ := openTestDB(t)
	defer db.Close()

	filter := struct {
		UserName string `db:"user_name"`
		Status   int
	}{
		UserName: "alice",
		Status:   1,
	}

	session := NewStdMySQL(db).
		Select("id").
		From("users").
		AppendRawSelective("WHERE user_name = #{user_name} AND status = #{status}", filter)

	snap := session.(interface{ snapshot() sessionSnapshot }).snapshot()
	if snap.sqlText != "SELECT id\nFROM users WHERE user_name = #{user_name} AND status = #{status}" {
		t.Fatalf("sqlText = %q", snap.sqlText)
	}
	if !reflect.DeepEqual(snap.argMap["#{user_name}"], "alice") {
		t.Fatalf("argMap[#{user_name}] = %#v, want %q", snap.argMap["#{user_name}"], "alice")
	}
	if !reflect.DeepEqual(snap.argMap["#{status}"], 1) {
		t.Fatalf("argMap[#{status}] = %#v, want 1", snap.argMap["#{status}"])
	}
}

func TestAppendRawSelectivePrunesNamedConditionWhenValueMissing(t *testing.T) {
	db, _ := openTestDB(t)
	defer db.Close()

	filter := struct {
		UserName string `db:"user_name"`
		Status   int
	}{
		UserName: "alice",
	}

	session := NewStdMySQL(db).
		Select("id").
		From("users").
		AppendRawSelective("WHERE user_name = #{user_name} AND status = #{status}", filter)

	snap := session.(interface{ snapshot() sessionSnapshot }).snapshot()
	if snap.err != nil {
		t.Fatalf("snapshot.err = %v, want nil", snap.err)
	}
	if snap.sqlText != "SELECT id\nFROM users WHERE user_name = #{user_name}" {
		t.Fatalf("sqlText = %q, want %q", snap.sqlText, "SELECT id\nFROM users WHERE user_name = #{user_name}")
	}
	if !reflect.DeepEqual(snap.argMap, map[string]any{"#{user_name}": "alice"}) {
		t.Fatalf("argMap = %#v, want %#v", snap.argMap, map[string]any{"#{user_name}": "alice"})
	}
}

func TestAppendRawSelectiveBindsPositionalArgsWhenAllPresent(t *testing.T) {
	db, _ := openTestDB(t)
	defer db.Close()

	session := NewStdMySQL(db).
		Select("id").
		From("users").
		AppendRawSelective("WHERE status = #{status} AND role = #{role}", 1, "admin")

	snap := session.(interface{ snapshot() sessionSnapshot }).snapshot()
	if snap.sqlText != "SELECT id\nFROM users WHERE status = #{status} AND role = #{role}" {
		t.Fatalf("sqlText = %q", snap.sqlText)
	}
	if !reflect.DeepEqual(snap.argMap["#{status}"], 1) {
		t.Fatalf("argMap[#{status}] = %#v, want 1", snap.argMap["#{status}"])
	}
	if !reflect.DeepEqual(snap.argMap["#{role}"], "admin") {
		t.Fatalf("argMap[#{role}] = %#v, want %q", snap.argMap["#{role}"], "admin")
	}
}

func TestAppendRawSelectivePrunesPositionalConditionWhenValueMissing(t *testing.T) {
	db, _ := openTestDB(t)
	defer db.Close()

	session := NewStdMySQL(db).
		Select("id").
		From("users").
		AppendRawSelective("WHERE status = #{status} AND role = #{role}", 1, "")

	snap := session.(interface{ snapshot() sessionSnapshot }).snapshot()
	if snap.err != nil {
		t.Fatalf("snapshot.err = %v, want nil", snap.err)
	}
	if snap.sqlText != "SELECT id\nFROM users WHERE status = #{status}" {
		t.Fatalf("sqlText = %q, want %q", snap.sqlText, "SELECT id\nFROM users WHERE status = #{status}")
	}
	if !reflect.DeepEqual(snap.argMap, map[string]any{"#{status}": 1}) {
		t.Fatalf("argMap = %#v, want %#v", snap.argMap, map[string]any{"#{status}": 1})
	}
}

func TestAppendRawSelectivePrunesOnlyMissingNamedBranches(t *testing.T) {
	db, _ := openTestDB(t)
	defer db.Close()

	filter := map[string]any{
		"b": "two",
		"c": "three",
	}

	session := NewStdMySQL(db).
		Select("id").
		From("users").
		AppendRawSelective("WHERE a = #{a} AND b = #{b} OR c = #{c}", filter)

	snap := session.(interface{ snapshot() sessionSnapshot }).snapshot()
	if snap.err != nil {
		t.Fatalf("snapshot.err = %v, want nil", snap.err)
	}
	if snap.sqlText != "SELECT id\nFROM users WHERE b = #{b} OR c = #{c}" {
		t.Fatalf("sqlText = %q, want %q", snap.sqlText, "SELECT id\nFROM users WHERE b = #{b} OR c = #{c}")
	}
	if !reflect.DeepEqual(snap.argMap, map[string]any{"#{b}": "two", "#{c}": "three"}) {
		t.Fatalf("argMap = %#v", snap.argMap)
	}
}

func TestWhereSelectiveSupportsPresentZeroValue(t *testing.T) {
	db, _ := openTestDB(t)
	defer db.Close()

	session := NewStdMySQL(db).
		Select("id").
		From("users").
		WhereSelective("status = #{status}", Present(0))

	snap := session.(interface{ snapshot() sessionSnapshot }).snapshot()
	if snap.sqlText != "SELECT id\nFROM users\nWHERE (status = #{status})" {
		t.Fatalf("sqlText = %q", snap.sqlText)
	}
	if !reflect.DeepEqual(snap.argMap, map[string]any{"#{status}": 0}) {
		t.Fatalf("argMap = %#v", snap.argMap)
	}
}

func TestSetSelectiveSupportsPresentFalseValue(t *testing.T) {
	db, _ := openTestDB(t)
	defer db.Close()

	session := NewStdMySQL(db).
		Update("users").
		SetSelective("enabled", Present(false))

	snap := session.(interface{ snapshot() sessionSnapshot }).snapshot()
	if snap.sqlText != "UPDATE users\nSET enabled = #{enabled_0}" {
		t.Fatalf("sqlText = %q", snap.sqlText)
	}
	if !reflect.DeepEqual(snap.argMap, map[string]any{"#{enabled_0}": false}) {
		t.Fatalf("argMap = %#v", snap.argMap)
	}
}

func TestValuesSelectiveSupportsPresentNilValue(t *testing.T) {
	db, _ := openTestDB(t)
	defer db.Close()

	session := NewStdMySQL(db).
		InsertInto("users").
		ValuesSelective("deleted_at", Present[any](nil))

	snap := session.(interface{ snapshot() sessionSnapshot }).snapshot()
	if snap.sqlText != "INSERT INTO users\n (deleted_at)\nVALUES (#{deleted_at_0})" {
		t.Fatalf("sqlText = %q", snap.sqlText)
	}
	value, ok := snap.argMap["#{deleted_at_0}"]
	if !ok {
		t.Fatalf("argMap = %#v, want deleted_at placeholder", snap.argMap)
	}
	if value != nil {
		t.Fatalf("argMap[#{deleted_at_0}] = %#v, want nil", value)
	}
}

func TestAddParamSelectiveSupportsPresentEmptyString(t *testing.T) {
	db, _ := openTestDB(t)
	defer db.Close()

	session := NewStdMySQL(db).
		Select("id").
		From("users").
		AddParamSelective("user_name", Present("")).
		AppendRaw("WHERE user_name = #{user_name}")

	snap := session.(interface{ snapshot() sessionSnapshot }).snapshot()
	if snap.sqlText != "SELECT id\nFROM users WHERE user_name = #{user_name}" {
		t.Fatalf("sqlText = %q", snap.sqlText)
	}
	if !reflect.DeepEqual(snap.argMap, map[string]any{"#{user_name}": ""}) {
		t.Fatalf("argMap = %#v", snap.argMap)
	}
}

func TestAppendRawSelectiveSupportsPresentZeroValue(t *testing.T) {
	db, _ := openTestDB(t)
	defer db.Close()

	session := NewStdMySQL(db).
		Select("id").
		From("users").
		AppendRawSelective("WHERE status = #{status}", Present(0))

	snap := session.(interface{ snapshot() sessionSnapshot }).snapshot()
	if snap.sqlText != "SELECT id\nFROM users WHERE status = #{status}" {
		t.Fatalf("sqlText = %q", snap.sqlText)
	}
	if !reflect.DeepEqual(snap.argMap, map[string]any{"#{status}": 0}) {
		t.Fatalf("argMap = %#v", snap.argMap)
	}
}

func TestAppendRawSelectivePrunesPresentNullOptional(t *testing.T) {
	db, _ := openTestDB(t)
	defer db.Close()

	filter := struct {
		Status Optional[*int]
		Name   string
	}{
		Status: Present[*int](nil),
		Name:   "alice",
	}

	session := NewStdMySQL(db).
		Select("id").
		From("users").
		AppendRawSelective("WHERE status = #{status} AND name = #{name}", filter)

	snap := session.(interface{ snapshot() sessionSnapshot }).snapshot()
	if snap.sqlText != "SELECT id\nFROM users WHERE name = #{name}" {
		t.Fatalf("sqlText = %q", snap.sqlText)
	}
	if !reflect.DeepEqual(snap.argMap, map[string]any{"#{name}": "alice"}) {
		t.Fatalf("argMap = %#v", snap.argMap)
	}
}

func TestAppendRawSelectiveKeepsBetweenClauseIntact(t *testing.T) {
	db, _ := openTestDB(t)
	defer db.Close()

	session := NewStdMySQL(db).
		Select("id").
		From("users").
		AppendRawSelective(
			"WHERE created_at BETWEEN #{from} AND #{to} AND status = #{status}",
			"2024-01-01",
			"2024-02-01",
			"",
		)

	snap := session.(interface{ snapshot() sessionSnapshot }).snapshot()
	if snap.err != nil {
		t.Fatalf("snapshot.err = %v, want nil", snap.err)
	}
	if snap.sqlText != "SELECT id\nFROM users WHERE created_at BETWEEN #{from} AND #{to}" {
		t.Fatalf("sqlText = %q", snap.sqlText)
	}
	if !reflect.DeepEqual(snap.argMap, map[string]any{"#{from}": "2024-01-01", "#{to}": "2024-02-01"}) {
		t.Fatalf("argMap = %#v", snap.argMap)
	}
}

func TestAppendRawNamedUnwrapsPresentOptionalFields(t *testing.T) {
	db, _ := openTestDB(t)
	defer db.Close()

	filter := struct {
		Status Optional[int]
	}{
		Status: Present(0),
	}

	session := NewStdMySQL(db).
		Select("id").
		From("users").
		AppendRawNamed("WHERE status = #{status}", filter)

	snap := session.(interface{ snapshot() sessionSnapshot }).snapshot()
	if snap.sqlText != "SELECT id\nFROM users WHERE status = #{status}" {
		t.Fatalf("sqlText = %q", snap.sqlText)
	}
	if !reflect.DeepEqual(snap.argMap, map[string]any{"#{status}": 0}) {
		t.Fatalf("argMap = %#v", snap.argMap)
	}
}

func TestAppendRawNamedRejectsAbsentOptionalFields(t *testing.T) {
	db, _ := openTestDB(t)
	defer db.Close()

	filter := struct {
		Status Optional[int]
	}{
		Status: Absent[int](),
	}

	session := NewStdMySQL(db).
		Select("id").
		From("users").
		AppendRawNamed("WHERE status = #{status}", filter)

	snap := session.(interface{ snapshot() sessionSnapshot }).snapshot()
	if snap.err == nil {
		t.Fatal("snapshot.err = nil, want error")
	}
	if !strings.Contains(snap.err.Error(), "missing named value for placeholder #{status}") {
		t.Fatalf("snapshot.err = %v", snap.err)
	}
}

func TestAppendRawNamedBindsStructFields(t *testing.T) {
	db, _ := openTestDB(t)
	defer db.Close()

	filter := struct {
		UserName string `db:"user_name"`
		Status   int
	}{
		UserName: "alice",
		Status:   1,
	}

	session := NewStdPostgres(db).
		Select("id").
		From("users").
		AppendRawNamed("WHERE user_name = #{user_name} AND status = #{status}", filter)

	snap := session.(interface{ snapshot() sessionSnapshot }).snapshot()
	if snap.sqlText != "SELECT id\nFROM users WHERE user_name = #{user_name} AND status = #{status}" {
		t.Fatalf("sqlText = %q", snap.sqlText)
	}
	if !reflect.DeepEqual(snap.argMap["#{user_name}"], "alice") {
		t.Fatalf("argMap[#{user_name}] = %#v, want %q", snap.argMap["#{user_name}"], "alice")
	}
	if !reflect.DeepEqual(snap.argMap["#{status}"], 1) {
		t.Fatalf("argMap[#{status}] = %#v, want 1", snap.argMap["#{status}"])
	}
}

func TestAppendRawNamedBindsMapKeysCaseInsensitively(t *testing.T) {
	db, _ := openTestDB(t)
	defer db.Close()

	session := NewStdMySQL(db).
		Select("id").
		From("users").
		AppendRawNamed("WHERE user_name = #{user_name}", map[string]any{"USER_NAME": "alice"})

	snap := session.(interface{ snapshot() sessionSnapshot }).snapshot()
	if !reflect.DeepEqual(snap.argMap["#{user_name}"], "alice") {
		t.Fatalf("argMap[#{user_name}] = %#v, want %q", snap.argMap["#{user_name}"], "alice")
	}
}

func TestAppendRawNamedRejectsUnsupportedSource(t *testing.T) {
	db, _ := openTestDB(t)
	defer db.Close()

	session := NewStdMySQL(db).AppendRawNamed("WHERE id = #{id}", 1)
	snap := session.(interface{ snapshot() sessionSnapshot }).snapshot()

	if snap.err == nil {
		t.Fatal("snapshot.err = nil, want error")
	}
	if !strings.Contains(snap.err.Error(), "named arg source must be a struct or map with string keys") {
		t.Fatalf("snapshot.err = %v", snap.err)
	}
}

func TestStdTransactionCommits(t *testing.T) {
	db, state := openTestDB(t)
	defer db.Close()

	err := NewStdMySQL(db).Transaction(func(tx Session) error {
		_, err := tx.InsertInto("audit_logs").Values("message", "created").Exec()
		return err
	})
	if err != nil {
		t.Fatalf("Transaction() error = %v", err)
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if state.beginCount != 1 {
		t.Fatalf("beginCount = %d, want 1", state.beginCount)
	}
	if state.commitCount != 1 {
		t.Fatalf("commitCount = %d, want 1", state.commitCount)
	}
	if state.rollbackCount != 0 {
		t.Fatalf("rollbackCount = %d, want 0", state.rollbackCount)
	}
}

type testCall struct {
	query string
	args  []any
}

type testDriverState struct {
	mu                  sync.Mutex
	queryColumns        []string
	queryRows           [][]driver.Value
	queryCalls          []testCall
	execCalls           []testCall
	execRowsAffected    int64
	execLastInsertID    int64
	supportLastInsertID bool
	beginCount          int
	commitCount         int
	rollbackCount       int
}

func (s *testDriverState) setRows(columns []string, rows [][]driver.Value) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queryColumns = append([]string(nil), columns...)
	s.queryRows = cloneDriverRows(rows)
}

func cloneDriverRows(rows [][]driver.Value) [][]driver.Value {
	out := make([][]driver.Value, len(rows))
	for i, row := range rows {
		out[i] = append([]driver.Value(nil), row...)
	}
	return out
}

var (
	registerTestDriver sync.Once
	testDriverCounter  int
)

func openTestDB(t *testing.T) (*sql.DB, *testDriverState) {
	t.Helper()

	state := &testDriverState{
		queryColumns:        []string{"id"},
		queryRows:           [][]driver.Value{{int64(1)}},
		execRowsAffected:    1,
		execLastInsertID:    1,
		supportLastInsertID: true,
	}

	registerTestDriver.Do(func() {
		sql.Register("sqlcraft-test", &testDriver{})
	})

	testDriverCounter++
	dsn := "state-" + strconv.Itoa(testDriverCounter)
	testDriverStates.Store(dsn, state)

	db, err := sql.Open("sqlcraft-test", dsn)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	return db, state
}

var testDriverStates sync.Map

type testDriver struct{}

func (d *testDriver) Open(name string) (driver.Conn, error) {
	value, _ := testDriverStates.Load(name)
	return &testConn{state: value.(*testDriverState)}, nil
}

type testConn struct {
	state *testDriverState
}

func (c *testConn) Prepare(string) (driver.Stmt, error) {
	return nil, driver.ErrSkip
}

func (c *testConn) Close() error {
	return nil
}

func (c *testConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *testConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.state.mu.Lock()
	c.state.beginCount++
	c.state.mu.Unlock()
	return &testTx{state: c.state}, nil
}

func (c *testConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.state.mu.Lock()
	c.state.queryCalls = append(c.state.queryCalls, testCall{query: query, args: namedValuesToAny(args)})
	columns := append([]string(nil), c.state.queryColumns...)
	rows := cloneDriverRows(c.state.queryRows)
	c.state.mu.Unlock()

	return &testRows{columns: columns, rows: rows}, nil
}

func (c *testConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	c.state.execCalls = append(c.state.execCalls, testCall{query: query, args: namedValuesToAny(args)})
	return testResult{
		rowsAffected:        c.state.execRowsAffected,
		lastInsertID:        c.state.execLastInsertID,
		supportLastInsertID: c.state.supportLastInsertID,
	}, nil
}

type testTx struct {
	state *testDriverState
}

func (tx *testTx) Commit() error {
	tx.state.mu.Lock()
	tx.state.commitCount++
	tx.state.mu.Unlock()
	return nil
}

func (tx *testTx) Rollback() error {
	tx.state.mu.Lock()
	tx.state.rollbackCount++
	tx.state.mu.Unlock()
	return nil
}

type testRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

func (r *testRows) Columns() []string {
	return append([]string(nil), r.columns...)
}

func (r *testRows) Close() error {
	return nil
}

func (r *testRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

func namedValuesToAny(args []driver.NamedValue) []any {
	values := make([]any, len(args))
	for i, arg := range args {
		values[i] = arg.Value
	}
	return values
}

func assertIntArgs(t *testing.T, got []any, want ...int64) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("args len = %d, want %d (%#v)", len(got), len(want), got)
	}
	for i, value := range got {
		gotInt, err := toInt64(value)
		if err != nil {
			t.Fatalf("arg[%d] = %#v is not integer-like: %v", i, value, err)
		}
		if gotInt != want[i] {
			t.Fatalf("arg[%d] = %d, want %d (all args: %#v)", i, gotInt, want[i], got)
		}
	}
}

type testResult struct {
	rowsAffected        int64
	lastInsertID        int64
	supportLastInsertID bool
}

func (r testResult) LastInsertId() (int64, error) {
	if !r.supportLastInsertID {
		return 0, driver.ErrSkip
	}
	return r.lastInsertID, nil
}

func (r testResult) RowsAffected() (int64, error) {
	return r.rowsAffected, nil
}
