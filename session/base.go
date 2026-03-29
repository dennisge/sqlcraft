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
	"database/sql"
	"reflect"
	"strconv"
	"strings"

	"github.com/dennisge/sqlcraft/sqltext"
)

type sessionBackend interface {
	scan(sqlText string, argMap map[string]any, debug bool, dest any) error
	execResult(sqlText string, argMap map[string]any, debug bool) (ExecutionResult, error)
	transaction(fc func(sessionBackend) error, opts ...*sql.TxOptions) error
	renew() sessionBackend
}

type sessionSnapshot struct {
	sqlText string
	argMap  map[string]any
	err     error
}

type baseSession struct {
	sql      sqltext.Builder
	argMap   map[string]any
	rawSQL   []string
	backend  sessionBackend
	self     Session
	debug    bool
	err      error
	paramSeq int
}

func newBaseSession(backend sessionBackend) *baseSession {
	return &baseSession{
		sql:     sqltext.New(),
		argMap:  make(map[string]any),
		rawSQL:  make([]string, 0),
		backend: backend,
	}
}

func (s *baseSession) Select(columns ...string) Session {
	s.sql.Select(columns...)
	return s.current()
}

func (s *baseSession) From(tables ...string) Session {
	s.sql.From(tables...)
	return s.current()
}

func (s *baseSession) Where(condition string, args ...any) Session {
	if len(args) > 0 {
		if err := s.bindArgs(condition, args, "where"); err != nil {
			s.err = err
			return s.current()
		}
	}
	s.sql.Where(condition)
	return s.current()
}

func (s *baseSession) WhereSelective(condition string, arg any) Session {
	if !isNotZero(arg) {
		return s.current()
	}
	if err := s.fillArgValue(condition, arg); err != nil {
		s.err = sessionWrapError("where selective", err)
		return s.current()
	}
	s.sql.Where(condition)
	return s.current()
}

func (s *baseSession) WhereIn(column string, args []any) Session {
	if len(args) == 0 {
		return s.current()
	}
	b := strings.Builder{}
	b.WriteString(column)
	b.WriteString(" IN (")
	for i, arg := range args {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(s.newDynamicPlaceholder(column, arg))
	}
	b.WriteString(")")
	s.sql.Where(b.String())
	return s.current()
}

func (s *baseSession) WhereNotIn(column string, args []any) Session {
	if len(args) == 0 {
		return s.current()
	}
	b := strings.Builder{}
	b.WriteString(column)
	b.WriteString(" NOT IN (")
	for i, arg := range args {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(s.newDynamicPlaceholder(column, arg))
	}
	b.WriteString(")")
	s.sql.Where(b.String())
	return s.current()
}

func (s *baseSession) WhereInInt64(column string, args []int64) Session {
	return s.WhereIn(column, int64ToAny(args))
}

func (s *baseSession) WhereNotInInt64(column string, args []int64) Session {
	return s.WhereNotIn(column, int64ToAny(args))
}

func (s *baseSession) GroupBy(columns ...string) Session {
	s.sql.GroupBy(columns...)
	return s.current()
}

func (s *baseSession) Having(condition string, value any) Session {
	if err := s.fillArgValue(condition, value); err != nil {
		s.err = sessionWrapError("having", err)
		return s.current()
	}
	s.sql.Having(condition)
	return s.current()
}

func (s *baseSession) OrderBy(columns ...string) Session {
	s.sql.OrderBy(columns...)
	return s.current()
}

func (s *baseSession) InsertInto(table string) Session {
	s.sql.InsertInto(table)
	return s.current()
}

func (s *baseSession) Values(column string, value any) Session {
	s.sql.Values(column, s.newDynamicPlaceholder(column, value))
	return s.current()
}

func (s *baseSession) ValuesSelective(column string, value any) Session {
	if isNotZero(value) {
		s.sql.Values(column, s.newDynamicPlaceholder(column, value))
	}
	return s.current()
}

func (s *baseSession) IntoColumns(columns ...string) Session {
	s.sql.IntoColumns(columns...)
	return s.current()
}

func (s *baseSession) IntoValues(values ...any) Session {
	items := make([]string, len(values))
	for i, value := range values {
		items[i] = s.newDynamicPlaceholder("value", value)
	}
	s.sql.IntoValues(items...)
	return s.current()
}

func (s *baseSession) IntoMultiValues(values [][]any) Session {
	if values == nil {
		return s.current()
	}
	for i, row := range values {
		if i > 0 {
			s.sql.AddRow()
		}
		items := make([]string, len(row))
		for j, value := range row {
			items[j] = s.newDynamicPlaceholder("value", value)
		}
		s.sql.IntoValues(items...)
	}
	return s.current()
}

func (s *baseSession) Update(table string) Session {
	s.sql.Update(table)
	return s.current()
}

func (s *baseSession) Set(column string, value any) Session {
	s.sql.Set(column + " = " + s.newDynamicPlaceholder(column, value))
	return s.current()
}

func (s *baseSession) SetSelective(column string, value any) Session {
	if isNotZero(value) {
		s.sql.Set(column + " = " + s.newDynamicPlaceholder(column, value))
	}
	return s.current()
}

func (s *baseSession) DeleteFrom(table string) Session {
	s.sql.DeleteFrom(table)
	return s.current()
}

func (s *baseSession) InnerJoin(joins ...string) Session {
	s.sql.InnerJoin(joins...)
	return s.current()
}

func (s *baseSession) InnerJoinSelective(join string, condition any) Session {
	if isNotZero(condition) {
		s.sql.InnerJoin(join)
	}
	return s.current()
}

func (s *baseSession) LeftOuterJoin(joins ...string) Session {
	s.sql.LeftOuterJoin(joins...)
	return s.current()
}

func (s *baseSession) RightOuterJoin(joins ...string) Session {
	s.sql.RightOuterJoin(joins...)
	return s.current()
}

func (s *baseSession) OuterJoin(joins ...string) Session {
	s.sql.OuterJoin(joins...)
	return s.current()
}

func (s *baseSession) Or() Session {
	s.sql.Or()
	return s.current()
}

func (s *baseSession) And() Session {
	s.sql.And()
	return s.current()
}

func (s *baseSession) Limit(limit int) Session {
	s.sql.Limit(s.newDynamicPlaceholder("limit", limit))
	return s.current()
}

func (s *baseSession) Offset(offset int) Session {
	s.sql.Offset(s.newDynamicPlaceholder("offset", offset))
	return s.current()
}

func (s *baseSession) Returning(columns ...string) Session {
	s.sql.Returning(columns...)
	return s.current()
}

func (s *baseSession) AddParam(param string, value any) Session {
	if err := s.bindPlaceholder(normalizeParam(param), value); err != nil {
		s.err = sessionWrapError("add param", err)
	}
	return s.current()
}

func (s *baseSession) AddParamSelective(param string, value any) Session {
	if !isNotZero(value) {
		return s.current()
	}
	if err := s.bindPlaceholder(normalizeParam(param), value); err != nil {
		s.err = sessionWrapError("add param selective", err)
	}
	return s.current()
}

func (s *baseSession) AppendRaw(rawSQL string, args ...any) Session {
	if len(args) > 0 {
		if err := s.bindArgs(rawSQL, args, "append raw"); err != nil {
			s.err = err
			return s.current()
		}
	}
	s.rawSQL = append(s.rawSQL, rawSQL)
	return s.current()
}

func (s *baseSession) Append(other Session) Session {
	snapper, ok := other.(interface{ snapshot() sessionSnapshot })
	if !ok {
		return s.current()
	}
	snap := snapper.snapshot()
	if snap.err != nil {
		if s.err == nil {
			s.err = snap.err
		}
		return s.current()
	}
	if snap.sqlText == "" {
		return s.current()
	}

	sqlText := snap.sqlText
	remapped := make(map[string]string)
	for _, ph := range getPlaceholders(sqlText) {
		target, ok := remapped[ph]
		if !ok {
			value, exists := snap.argMap[ph]
			if !exists {
				s.err = sessionErrorf("append: missing value for placeholder %s", ph)
				return s.current()
			}
			target = ph
			if current, exists := s.argMap[ph]; exists && !reflect.DeepEqual(current, value) {
				target = s.newPlaceholderLike(ph, value)
			} else if !exists {
				s.argMap[ph] = value
			}
			remapped[ph] = target
		}
		if target != ph {
			sqlText = strings.Replace(sqlText, ph, target, 1)
		}
	}

	s.rawSQL = append(s.rawSQL, sqlText)
	return s.current()
}

func (s *baseSession) Renew() Session {
	return newSessionForBackend(s.backend.renew())
}

func (s *baseSession) Debug() Session {
	s.debug = true
	return s.current()
}

func (s *baseSession) Scan(dest any) error {
	if s.err != nil {
		err := s.err
		s.Reset()
		return err
	}
	sqlText, argMap, debug := s.snapshotState()
	s.Reset()
	return s.backend.scan(sqlText, argMap, debug, dest)
}

func (s *baseSession) Exec() (int64, error) {
	result, err := s.ExecResult()
	if err != nil {
		return 0, err
	}
	return result.RowsAffected, nil
}

func (s *baseSession) ExecResult() (ExecutionResult, error) {
	if s.err != nil {
		err := s.err
		s.Reset()
		return ExecutionResult{}, err
	}
	sqlText, argMap, debug := s.snapshotState()
	s.Reset()
	return s.backend.execResult(sqlText, argMap, debug)
}

func (s *baseSession) Transaction(fc TxFunc, opts ...*sql.TxOptions) error {
	if s.err != nil {
		err := s.err
		s.Reset()
		return err
	}
	s.Reset()
	return s.backend.transaction(func(txBackend sessionBackend) error {
		return fc(newSessionForBackend(txBackend))
	}, opts...)
}

func (s *baseSession) Reset() {
	s.sql = sqltext.New()
	s.argMap = make(map[string]any)
	s.rawSQL = nil
	s.err = nil
	s.debug = false
	s.paramSeq = 0
}

func (s *baseSession) snapshot() sessionSnapshot {
	return sessionSnapshot{
		sqlText: s.getSQLText(),
		argMap:  cloneArgMap(s.argMap),
		err:     s.err,
	}
}

func (s *baseSession) snapshotState() (string, map[string]any, bool) {
	return s.getSQLText(), cloneArgMap(s.argMap), s.debug
}

func (s *baseSession) getSQLText() string {
	sqlText := s.sql.String()
	if len(sqlText) == 0 {
		return strings.Join(s.rawSQL, " ")
	}
	if len(s.rawSQL) > 0 {
		return sqlText + " " + strings.Join(s.rawSQL, " ")
	}
	return sqlText
}

func (s *baseSession) bindArgs(sqlText string, args []any, caller string) error {
	placeholders := getPlaceholders(sqlText)
	if len(args) != len(placeholders) {
		return sessionErrorf("%s: placeholder count %d != arg count %d in %q", caller, len(placeholders), len(args), sqlText)
	}
	for i, ph := range placeholders {
		if err := s.bindPlaceholder(ph, args[i]); err != nil {
			return sessionWrapError(caller, err)
		}
	}
	return nil
}

func (s *baseSession) fillArgValue(sqlText string, value any) error {
	for _, ph := range getPlaceholders(sqlText) {
		if err := s.bindPlaceholder(ph, value); err != nil {
			return err
		}
	}
	return nil
}

func (s *baseSession) bindPlaceholder(ph string, value any) error {
	if current, exists := s.argMap[ph]; exists {
		if reflect.DeepEqual(current, value) {
			return nil
		}
		return sessionErrorf("placeholder %s is already bound to a different value", ph)
	}
	s.argMap[ph] = value
	return nil
}

func (s *baseSession) newDynamicPlaceholder(base string, value any) string {
	return s.newPlaceholder("#", base, value)
}

func (s *baseSession) newPlaceholderLike(ph string, value any) string {
	prefix := "#"
	if strings.HasPrefix(ph, "${") {
		prefix = "$"
	}
	return s.newPlaceholder(prefix, placeholderName(ph), value)
}

func (s *baseSession) newPlaceholder(prefix, base string, value any) string {
	base = sanitizeParamName(base)
	if base == "" {
		base = "p"
	}
	for {
		candidate := prefix + "{" + base + "_" + strconv.Itoa(s.paramSeq) + "}"
		s.paramSeq++
		if _, exists := s.argMap[candidate]; exists {
			continue
		}
		s.argMap[candidate] = value
		return candidate
	}
}

func cloneArgMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func normalizeParam(param string) string {
	if strings.HasPrefix(param, "#{") || strings.HasPrefix(param, "${") {
		return param
	}
	return "#{" + param + "}"
}

func placeholderName(ph string) string {
	if len(ph) < 4 {
		return ""
	}
	return ph[2 : len(ph)-1]
}

func sanitizeParamName(name string) string {
	if name == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	return strings.Trim(b.String(), "_")
}

func int64ToAny(ids []int64) []any {
	out := make([]any, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}

func (s *baseSession) current() Session {
	if s.self != nil {
		return s.self
	}
	return s
}
