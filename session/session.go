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

// Package session provides a MyBatis-style fluent SQL session with multiple backend support.
//
// Placeholders use two syntaxes:
//   - #{name} — parameterized binding (safe, prevents SQL injection)
//   - ${name} — literal string injection (use ONLY for trusted column/table names, NEVER for user input)
//
// Supported backends:
//   - GORM: use [New] or [NewGorm] to create a GORM-backed session
//   - database/sql (MySQL): use [NewStdMySQL] to create a stdlib MySQL session
//   - database/sql (PostgreSQL): use [NewStdPostgres] to create a stdlib PostgreSQL session
//   - database/sql (generic): use [NewStd] with an explicit [Dialect]
//
// When you already know the database package at compile time, prefer the
// provider-specific helpers in [github.com/dennisge/sqlcraft/driver/mysql] or
// [github.com/dennisge/sqlcraft/driver/postgres], such as OpenSession or NewSession.
package session

import "database/sql"

// Use [Present], [Absent], or [Maybe] with Selective methods when zero values
// such as 0, false, "", or nil must still be treated as explicitly provided.
//
//	Tx.WhereSelective("status = #{status}", Present(0))
//	Tx.AppendRawSelective("AND deleted_at IS NULL", Present(true))
//
// TxFunc is the callback type for [Session.Transaction].
type TxFunc func(session Session) error

// Session is a fluent SQL builder with parameter binding and execution.
// It is NOT safe for concurrent use.
//
// A Session is typically used for one SQL statement at a time. After calling
// a terminal method ([Session.Scan], [Session.Exec]), the internal state is
// automatically reset so the Session can be reused.
//
// Session is backend-agnostic — the same interface works with GORM, database/sql
// for MySQL, or database/sql for PostgreSQL. Use [New], [NewGorm], [NewStd],
// [NewStdMySQL], or [NewStdPostgres] to create an appropriate implementation.
// For application code, the provider-specific helper packages usually offer a
// cleaner entry point because they can encode the dialect in the package API.
type Session interface {

	// ---- SELECT ----

	// Select sets the columns for a SELECT query.
	Select(columns ...string) Session

	// From sets the tables for a SELECT query.
	From(tables ...string) Session

	// ---- WHERE ----

	// Where adds a WHERE condition. Placeholders in condition are bound to args.
	Where(condition string, args ...any) Session

	// WhereSelective adds a WHERE condition only when arg is non-zero or explicitly present via [Present].
	// Explicit null optionals are pruned instead of producing "= NULL"; use an explicit
	// raw "IS NULL" or "IS NOT NULL" predicate when that SQL semantics is needed.
	WhereSelective(condition string, arg any) Session

	// WhereIn adds a WHERE column IN (...) condition when args is non-empty.
	WhereIn(column string, args []any) Session

	// WhereNotIn adds a WHERE column NOT IN (...) condition when args is non-empty.
	WhereNotIn(column string, args []any) Session

	// WhereInInt64 is a convenience wrapper around WhereIn for []int64.
	WhereInInt64(column string, args []int64) Session

	// WhereNotInInt64 is a convenience wrapper around WhereNotIn for []int64.
	WhereNotInInt64(column string, args []int64) Session

	// ---- GROUP / ORDER ----

	// GroupBy adds a GROUP BY clause.
	GroupBy(columns ...string) Session

	// Having adds a HAVING condition.
	Having(condition string, value any) Session

	// OrderBy adds an ORDER BY clause.
	OrderBy(columns ...string) Session

	// ---- INSERT ----

	// InsertInto sets the target table for an INSERT.
	InsertInto(table string) Session

	// Values adds a column-value pair for INSERT.
	Values(column string, value any) Session

	// ValuesSelective adds a column-value pair only when value is non-zero or explicitly present via [Present].
	ValuesSelective(column string, value any) Session

	// IntoColumns sets column names for bulk INSERT.
	IntoColumns(columns ...string) Session

	// IntoValues adds a row of values for bulk INSERT.
	IntoValues(values ...any) Session

	// IntoMultiValues adds multiple rows of values for bulk INSERT.
	IntoMultiValues(values [][]any) Session

	// ---- UPDATE ----

	// Update sets the target table for an UPDATE.
	Update(table string) Session

	// Set adds a column = value assignment for UPDATE.
	Set(column string, value any) Session

	// SetSelective adds a SET assignment only when value is non-zero or explicitly present via [Present].
	SetSelective(column string, value any) Session

	// ---- DELETE ----

	// DeleteFrom sets the target table for a DELETE.
	DeleteFrom(table string) Session

	// ---- JOIN ----

	// InnerJoin adds an INNER JOIN clause.
	InnerJoin(joins ...string) Session

	// InnerJoinSelective adds an INNER JOIN only when condition is non-zero or explicitly present via [Present].
	InnerJoinSelective(join string, condition any) Session

	// LeftOuterJoin adds a LEFT OUTER JOIN clause.
	LeftOuterJoin(joins ...string) Session

	// RightOuterJoin adds a RIGHT OUTER JOIN clause.
	RightOuterJoin(joins ...string) Session

	// OuterJoin adds an OUTER JOIN clause.
	OuterJoin(joins ...string) Session

	// ---- LOGICAL ----

	// Or inserts an OR between WHERE/HAVING conditions.
	Or() Session

	// And inserts an AND between WHERE/HAVING conditions.
	And() Session

	// ---- PAGINATION ----

	// Limit sets the LIMIT clause.
	Limit(limit int) Session

	// Offset sets the OFFSET clause.
	Offset(offset int) Session

	// Returning appends a RETURNING clause to the current DML statement.
	Returning(columns ...string) Session

	// ---- PARAMETERS ----

	// AddParam binds a named parameter value.
	//
	// #{param} uses parameterized binding (safe).
	// ${param} is injected literally — use ONLY for trusted column/table names.
	AddParam(param string, value any) Session

	// AddParamSelective binds a parameter only when value is non-zero or explicitly present via [Present].
	AddParamSelective(param string, value any) Session

	// ---- RAW SQL ----

	// AppendRaw appends raw SQL text after the builder-generated SQL.
	// When args are provided, placeholders are bound positionally in declaration order.
	AppendRaw(rawSQL string, args ...any) Session

	// AppendRawSelective appends raw SQL only when the required values are present.
	// With placeholders, values can come from positional args or a single struct/map source.
	// Boolean sub-conditions joined by AND/OR are pruned individually when their values are missing,
	// while predicates such as "BETWEEN ... AND ..." remain intact as one clause.
	// Use [Present] to keep zero values such as 0, false, "", or nil.
	// Explicit null optionals prune the corresponding predicate; use explicit "IS NULL"
	// SQL when you want to query for SQL null values.
	// Without placeholders, arg acts as a simple condition gate.
	AppendRawSelective(rawSQL string, arg any, args ...any) Session

	// AppendRawNamed appends raw SQL and strictly binds placeholders from a struct or map by placeholder name.
	// Missing values are treated as errors instead of skipping the fragment.
	// Explicitly present nil/null values are bound as SQL NULL, so callers should write
	// "IS NULL" themselves instead of relying on equality comparisons.
	AppendRawNamed(rawSQL string, arg any) Session

	// Append appends another Session's SQL to this one.
	Append(sql Session) Session

	// ---- LIFECYCLE ----

	// Renew creates a fresh Session sharing the same database connection and dialect.
	Renew() Session

	// Debug enables SQL logging for the next terminal operation.
	Debug() Session

	// ---- TERMINAL OPERATIONS ----

	// Scan executes a SELECT and scans results into dest (pointer to slice of structs).
	Scan(dest any) error

	// Exec executes an INSERT/UPDATE/DELETE and returns the number of affected rows.
	Exec() (int64, error)

	// ExecResult executes an INSERT/UPDATE/DELETE and returns rows affected plus generated ID metadata when available.
	ExecResult() (ExecutionResult, error)

	// Transaction executes fc within a database transaction.
	Transaction(fc TxFunc, opts ...*sql.TxOptions) error
}
