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

// Package sqltext provides a fluent API for building SQL text without parameter binding.
//
// It is the lowest-level building block in sqlcraft: it only cares about assembling
// syntactically correct SQL strings — parameter placeholders, escaping, and execution
// are handled by the higher-level [session] package.
package sqltext

import "strings"

const (
	and = ") AND ("
	or  = ") OR ("
)

// Builder is a fluent SQL text builder.
// It is NOT safe for concurrent use.
type Builder interface {
	Select(columns ...string)
	SelectDistinct(columns ...string)
	From(tables ...string)
	Update(table string)
	Set(sets ...string)
	InsertInto(table string)
	Values(columns string, values string)
	IntoColumns(columns ...string)
	IntoValues(values ...string)
	AddRow()
	DeleteFrom(table string)
	Join(joins ...string)
	InnerJoin(joins ...string)
	LeftOuterJoin(joins ...string)
	RightOuterJoin(joins ...string)
	OuterJoin(joins ...string)
	Where(conditions ...string)
	Or()
	And()
	GroupBy(columns ...string)
	Having(conditions ...string)
	OrderBy(columns ...string)
	Limit(limit string)
	Offset(offset string)
	FetchFirstRowsOnly(limit string)
	OffsetRows(offset string)
	Returning(columns ...string)
	Clone() Builder
	String() string
}

// New creates a new SQL text [Builder].
func New() Builder {
	return &builder{stmt: &statement{values: [][]string{{}}}}
}

type builder struct {
	stmt *statement
}

func (b *builder) Update(table string) {
	b.stmt.statementType = doUpdate
	b.stmt.tables = append(b.stmt.tables, table)
}

func (b *builder) Set(sets ...string) {
	b.stmt.sets = append(b.stmt.sets, sets...)
}

func (b *builder) InsertInto(table string) {
	b.stmt.statementType = doInsert
	b.stmt.tables = append(b.stmt.tables, table)
}

func (b *builder) Values(columns string, values string) {
	b.IntoColumns(columns)
	b.IntoValues(values)
}

func (b *builder) IntoColumns(columns ...string) {
	b.stmt.columns = append(b.stmt.columns, columns...)
}

func (b *builder) IntoValues(values ...string) {
	list := &b.stmt.values[len(b.stmt.values)-1]
	*list = append(*list, values...)
}

func (b *builder) AddRow() {
	b.stmt.values = append(b.stmt.values, make([]string, 0))
}

func (b *builder) Select(columns ...string) {
	b.stmt.statementType = doSelect
	b.stmt.selects = append(b.stmt.selects, columns...)
}

func (b *builder) SelectDistinct(columns ...string) {
	b.stmt.distinct = true
	b.Select(columns...)
}

func (b *builder) DeleteFrom(table string) {
	b.stmt.statementType = doDelete
	b.stmt.tables = append(b.stmt.tables, table)
}

func (b *builder) From(tables ...string) {
	b.stmt.tables = append(b.stmt.tables, tables...)
}

func (b *builder) Join(joins ...string) {
	b.stmt.join = append(b.stmt.join, joins...)
}

func (b *builder) InnerJoin(joins ...string) {
	b.stmt.innerJoin = append(b.stmt.innerJoin, joins...)
}

func (b *builder) LeftOuterJoin(joins ...string) {
	b.stmt.leftOuterJoin = append(b.stmt.leftOuterJoin, joins...)
}

func (b *builder) RightOuterJoin(joins ...string) {
	b.stmt.rightOuterJoin = append(b.stmt.rightOuterJoin, joins...)
}

func (b *builder) OuterJoin(joins ...string) {
	b.stmt.outerJoin = append(b.stmt.outerJoin, joins...)
}

func (b *builder) Where(conditions ...string) {
	b.stmt.where = append(b.stmt.where, conditions...)
	b.stmt.lastList = &b.stmt.where
}

func (b *builder) Or() {
	if b.stmt.lastList == nil {
		return
	}
	*b.stmt.lastList = append(*b.stmt.lastList, or)
}

func (b *builder) And() {
	if b.stmt.lastList == nil {
		return
	}
	*b.stmt.lastList = append(*b.stmt.lastList, and)
}

func (b *builder) GroupBy(columns ...string) {
	b.stmt.groupBy = append(b.stmt.groupBy, columns...)
}

func (b *builder) Having(conditions ...string) {
	b.stmt.having = append(b.stmt.having, conditions...)
	b.stmt.lastList = &b.stmt.having
}

func (b *builder) OrderBy(columns ...string) {
	b.stmt.orderBy = append(b.stmt.orderBy, columns...)
}

func (b *builder) Limit(limit string) {
	b.stmt.limit = limit
	b.stmt.limitingRowsStrategy = offsetLimit
}

func (b *builder) Offset(offset string) {
	b.stmt.offset = offset
	b.stmt.limitingRowsStrategy = offsetLimit
}

func (b *builder) FetchFirstRowsOnly(limit string) {
	b.stmt.limit = limit
	b.stmt.limitingRowsStrategy = iso
}

func (b *builder) OffsetRows(offset string) {
	b.stmt.offset = offset
	b.stmt.limitingRowsStrategy = iso
}

func (b *builder) Returning(columns ...string) {
	b.stmt.returning = append(b.stmt.returning, columns...)
}

func (b *builder) Clone() Builder {
	return &builder{stmt: b.stmt.clone()}
}

func (b *builder) String() string {
	sb := &strings.Builder{}
	b.stmt.sql(sb)
	return sb.String()
}
