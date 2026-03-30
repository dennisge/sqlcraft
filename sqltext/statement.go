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

package sqltext

import "strings"

type statementType int

const (
	doDelete statementType = iota
	doInsert
	doSelect
	doUpdate
)

type statement struct {
	statementType        statementType
	sets                 []string
	selects              []string
	tables               []string
	join                 []string
	innerJoin            []string
	outerJoin            []string
	leftOuterJoin        []string
	rightOuterJoin       []string
	where                []string
	having               []string
	groupBy              []string
	orderBy              []string
	lastList             *[]string
	columns              []string
	values               [][]string
	returning            []string
	distinct             bool
	offset               string
	limit                string
	limitingRowsStrategy limitingRowsStrategy
}

func (s *statement) clone() *statement {
	cloned := &statement{
		statementType:        s.statementType,
		sets:                 cloneStrings(s.sets),
		selects:              cloneStrings(s.selects),
		tables:               cloneStrings(s.tables),
		join:                 cloneStrings(s.join),
		innerJoin:            cloneStrings(s.innerJoin),
		outerJoin:            cloneStrings(s.outerJoin),
		leftOuterJoin:        cloneStrings(s.leftOuterJoin),
		rightOuterJoin:       cloneStrings(s.rightOuterJoin),
		where:                cloneStrings(s.where),
		having:               cloneStrings(s.having),
		groupBy:              cloneStrings(s.groupBy),
		orderBy:              cloneStrings(s.orderBy),
		columns:              cloneStrings(s.columns),
		values:               cloneStringMatrix(s.values),
		returning:            cloneStrings(s.returning),
		distinct:             s.distinct,
		offset:               s.offset,
		limit:                s.limit,
		limitingRowsStrategy: s.limitingRowsStrategy,
	}

	switch s.lastList {
	case &s.where:
		cloned.lastList = &cloned.where
	case &s.having:
		cloned.lastList = &cloned.having
	}

	return cloned
}

func cloneStrings(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	return append([]string(nil), src...)
}

func cloneStringMatrix(src [][]string) [][]string {
	if len(src) == 0 {
		return nil
	}
	out := make([][]string, len(src))
	for i, row := range src {
		out[i] = cloneStrings(row)
	}
	return out
}

func (s *statement) sql(sb *strings.Builder) {
	switch s.statementType {
	case doSelect:
		s.selectSQL(sb)
	case doDelete:
		s.deleteSQL(sb)
	case doInsert:
		s.insertSQL(sb)
	case doUpdate:
		s.updateSQL(sb)
	}
}

func (s *statement) selectSQL(sb *strings.Builder) {
	if s.distinct {
		s.clause(sb, "SELECT DISTINCT", s.selects, "", "", ", ")
	} else {
		s.clause(sb, "SELECT", s.selects, "", "", ", ")
	}
	s.clause(sb, "FROM", s.tables, "", "", ", ")
	s.joins(sb)
	s.clause(sb, "WHERE", s.where, "(", ")", " AND ")
	s.clause(sb, "GROUP BY", s.groupBy, "", "", ", ")
	s.clause(sb, "HAVING", s.having, "(", ")", " AND ")
	s.clause(sb, "ORDER BY", s.orderBy, "", "", ", ")
	s.limitingRowsStrategy.appendClause(sb, s.offset, s.limit)
}

func (s *statement) deleteSQL(sb *strings.Builder) {
	s.clause(sb, "DELETE FROM", s.tables, "", "", "")
	s.clause(sb, "WHERE", s.where, "(", ")", " AND ")
	s.limitingRowsStrategy.appendClause(sb, "", s.limit)
	s.clause(sb, "RETURNING", s.returning, "", "", ", ")
}

func (s *statement) insertSQL(sb *strings.Builder) {
	s.clause(sb, "INSERT INTO", s.tables, "", "", "")
	s.clause(sb, "", s.columns, "(", ")", ", ")
	for i, value := range s.values {
		keyword := "VALUES"
		if i > 0 {
			keyword = ","
		}
		s.clause(sb, keyword, value, "(", ")", ", ")
	}
	s.clause(sb, "RETURNING", s.returning, "", "", ", ")
}

func (s *statement) updateSQL(sb *strings.Builder) {
	s.clause(sb, "UPDATE", s.tables, "", "", "")
	s.joins(sb)
	s.clause(sb, "SET", s.sets, "", "", ", ")
	s.clause(sb, "WHERE", s.where, "(", ")", " AND ")
	s.limitingRowsStrategy.appendClause(sb, "", s.limit)
	s.clause(sb, "RETURNING", s.returning, "", "", ", ")
}

func (s *statement) clause(sb *strings.Builder, keyword string, parts []string, open, close, conjunction string) {
	if len(parts) == 0 {
		return
	}
	if sb.Len() > 0 {
		sb.WriteString("\n")
	}
	sb.WriteString(keyword)
	sb.WriteString(" ")
	sb.WriteString(open)
	last := "________"
	for i, part := range parts {
		if i > 0 && part != and && part != or && last != and && last != or {
			sb.WriteString(conjunction)
		}
		sb.WriteString(part)
		last = part
	}
	sb.WriteString(close)
}

func (s *statement) joins(sb *strings.Builder) {
	s.clause(sb, "JOIN", s.join, "", "", " JOIN ")
	s.clause(sb, "INNER JOIN", s.innerJoin, "", "", " INNER JOIN ")
	s.clause(sb, "OUTER JOIN", s.outerJoin, "", "", " OUTER JOIN ")
	s.clause(sb, "LEFT OUTER JOIN", s.leftOuterJoin, "", "", " LEFT OUTER JOIN ")
	s.clause(sb, "RIGHT OUTER JOIN", s.rightOuterJoin, "", "", " RIGHT OUTER JOIN ")
}

type limitingRowsStrategy int

const (
	nop limitingRowsStrategy = iota
	iso
	offsetLimit
)

func (ls limitingRowsStrategy) appendClause(sb *strings.Builder, offset, limit string) {
	switch ls {
	case offsetLimit:
		if limit != "" {
			sb.WriteString(" LIMIT ")
			sb.WriteString(limit)
		}
		if offset != "" {
			sb.WriteString(" OFFSET ")
			sb.WriteString(offset)
		}
	case iso:
		if offset != "" {
			sb.WriteString(" OFFSET ")
			sb.WriteString(offset)
			sb.WriteString(" ROWS")
		}
		if limit != "" {
			sb.WriteString(" FETCH FIRST ")
			sb.WriteString(limit)
			sb.WriteString(" ROWS ONLY")
		}
	case nop:
	}
}
