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
	"errors"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"
)

// StdSession is a [Session] implementation backed by database/sql.
type StdSession struct {
	*baseSession
}

type stdBackend struct {
	db      *sql.DB
	tx      *sql.Tx
	dialect Dialect
}

type sqlQueryExec interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func (b *stdBackend) scan(ctx context.Context, sqlText string, argMap map[string]any, debug bool, dest any) error {
	query, args, err := renderSQL(sqlText, argMap, func(_ string, value any, index int) (string, any, error) {
		ph, err := b.dialect.placeholder(index)
		return ph, value, err
	})
	if err != nil {
		return err
	}
	if debug {
		log.Printf("sqlcraft [%s] QUERY %s | args=%v", b.dialect, query, args)
	}

	rows, err := b.conn().QueryContext(sessionContext(ctx), query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	if err := scanRows(rows, dest); err != nil {
		return err
	}
	return rows.Err()
}

func (b *stdBackend) execResult(ctx context.Context, sqlText string, argMap map[string]any, debug bool) (ExecutionResult, error) {
	query, args, err := renderSQL(sqlText, argMap, func(_ string, value any, index int) (string, any, error) {
		ph, err := b.dialect.placeholder(index)
		return ph, value, err
	})
	if err != nil {
		return ExecutionResult{}, err
	}
	if debug {
		log.Printf("sqlcraft [%s] EXEC %s | args=%v", b.dialect, query, args)
	}

	result, err := b.conn().ExecContext(sessionContext(ctx), query, args...)
	if err != nil {
		return ExecutionResult{}, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return ExecutionResult{}, err
	}
	out := ExecutionResult{RowsAffected: rowsAffected}
	lastInsertID, err := result.LastInsertId()
	if err == nil {
		out = withLastInsertID(out, lastInsertID)
	} else {
		out = withLastInsertIDError(out, ErrLastInsertIDUnsupported)
	}
	return out, nil
}

func (b *stdBackend) transaction(ctx context.Context, fc func(sessionBackend) error, opts ...*sql.TxOptions) error {
	if b.tx != nil {
		return errors.New("session: nested transactions are not supported for the database/sql provider")
	}

	tx, err := b.db.BeginTx(sessionContext(ctx), firstTxOptions(opts))
	if err != nil {
		return err
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				panic(errors.Join(sessionErrorf("transaction panic: %v", recovered), rollbackErr))
			}
			panic(recovered)
		}
	}()

	if err := fc(&stdBackend{db: b.db, tx: tx, dialect: b.dialect}); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return errors.Join(err, rollbackErr)
		}
		return err
	}

	return tx.Commit()
}

func (b *stdBackend) renew() sessionBackend {
	return &stdBackend{
		db:      b.db,
		tx:      b.tx,
		dialect: b.dialect,
	}
}

func (b *stdBackend) conn() sqlQueryExec {
	if b.tx != nil {
		return b.tx
	}
	return b.db
}

func scanRows(rows *sql.Rows, dest any) error {
	if dest == nil {
		return errors.New("session: dest cannot be nil")
	}

	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return errors.New("session: dest must be a non-nil pointer")
	}

	target := rv.Elem()
	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	switch target.Kind() {
	case reflect.Slice:
		return scanRowsIntoSlice(rows, columns, target)
	case reflect.Struct:
		return scanRowIntoValue(rows, columns, target)
	case reflect.Map:
		return scanRowIntoValue(rows, columns, target)
	default:
		return errors.New("session: dest must point to a struct, map, or slice")
	}
}

func scanRowsIntoSlice(rows *sql.Rows, columns []string, target reflect.Value) error {
	elemType := target.Type().Elem()
	isPtr := elemType.Kind() == reflect.Pointer
	if isPtr {
		elemType = elemType.Elem()
	}

	for rows.Next() {
		item := reflect.New(elemType).Elem()
		if err := fillValueFromRow(rows, columns, item); err != nil {
			return err
		}
		if isPtr {
			ptr := reflect.New(elemType)
			ptr.Elem().Set(item)
			target.Set(reflect.Append(target, ptr))
		} else {
			target.Set(reflect.Append(target, item))
		}
	}
	return nil
}

func scanRowIntoValue(rows *sql.Rows, columns []string, target reflect.Value) error {
	if !rows.Next() {
		return nil
	}
	return fillValueFromRow(rows, columns, target)
}

func fillValueFromRow(rows *sql.Rows, columns []string, target reflect.Value) error {
	values := make([]any, len(columns))
	scanTargets := make([]any, len(columns))
	for i := range values {
		scanTargets[i] = &values[i]
	}
	if err := rows.Scan(scanTargets...); err != nil {
		return err
	}

	switch target.Kind() {
	case reflect.Struct:
		return assignStructColumns(target, columns, values)
	case reflect.Map:
		return assignMapColumns(target, columns, values)
	default:
		if len(values) != 1 {
			return fmt.Errorf("session: cannot scan %d columns into %s", len(values), target.Type())
		}
		return assignValue(target, values[0])
	}
}

func assignStructColumns(target reflect.Value, columns []string, values []any) error {
	fields := buildFieldIndex(target.Type())
	for i, column := range columns {
		index, ok := fields[strings.ToLower(column)]
		if !ok {
			continue
		}
		field, err := fieldByIndexAlloc(target, index)
		if err != nil {
			return sessionWrapError(fmt.Sprintf("assign column %q", column), err)
		}
		if err := assignValue(field, values[i]); err != nil {
			return sessionWrapError(fmt.Sprintf("assign column %q", column), err)
		}
	}
	return nil
}

func assignMapColumns(target reflect.Value, columns []string, values []any) error {
	if target.IsNil() {
		target.Set(reflect.MakeMap(target.Type()))
	}
	if target.Type().Key().Kind() != reflect.String {
		return errors.New("session: map destinations must use string keys")
	}
	for i, column := range columns {
		value := reflect.New(target.Type().Elem()).Elem()
		if err := assignValue(value, values[i]); err != nil {
			return sessionWrapError(fmt.Sprintf("assign column %q", column), err)
		}
		target.SetMapIndex(reflect.ValueOf(column), value)
	}
	return nil
}

func buildFieldIndex(typ reflect.Type) map[string][]int {
	indexes := make(map[string][]int)
	walkFields(typ, nil, indexes)
	return indexes
}

func fieldByIndexAlloc(value reflect.Value, index []int) (reflect.Value, error) {
	current := value
	for i, part := range index {
		if current.Kind() == reflect.Pointer {
			if current.IsNil() {
				current.Set(reflect.New(current.Type().Elem()))
			}
			current = current.Elem()
		}
		if current.Kind() != reflect.Struct {
			return reflect.Value{}, sessionErrorf("invalid field path for %s", value.Type())
		}
		field := current.Field(part)
		if i == len(index)-1 {
			return field, nil
		}
		if field.Kind() == reflect.Pointer && field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		current = field
	}
	return current, nil
}

func walkFields(typ reflect.Type, prefix []int, indexes map[string][]int) {
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}

		path := append(append([]int(nil), prefix...), i)
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			walkFields(field.Type, path, indexes)
			continue
		}
		if field.Anonymous && field.Type.Kind() == reflect.Pointer && field.Type.Elem().Kind() == reflect.Struct {
			walkFields(field.Type.Elem(), path, indexes)
			continue
		}

		for _, key := range fieldKeys(field) {
			if _, exists := indexes[key]; !exists {
				indexes[key] = path
			}
		}
	}
}

func fieldKeys(field reflect.StructField) []string {
	keys := []string{
		strings.ToLower(field.Name),
		strings.ToLower(toSnakeCase(field.Name)),
	}

	if dbTag := field.Tag.Get("db"); dbTag != "" && dbTag != "-" {
		keys = append(keys, strings.ToLower(strings.Split(dbTag, ",")[0]))
	}
	if jsonTag := field.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
		keys = append(keys, strings.ToLower(strings.Split(jsonTag, ",")[0]))
	}
	if gormTag := field.Tag.Get("gorm"); gormTag != "" {
		for _, part := range strings.Split(gormTag, ";") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "column:") {
				keys = append(keys, strings.ToLower(strings.TrimPrefix(part, "column:")))
			}
		}
	}

	return keys
}

func assignValue(target reflect.Value, value any) error {
	if !target.CanSet() {
		return nil
	}
	if value == nil {
		target.SetZero()
		return nil
	}

	if target.Kind() == reflect.Pointer {
		elem := reflect.New(target.Type().Elem()).Elem()
		if err := assignValue(elem, value); err != nil {
			return err
		}
		ptr := reflect.New(target.Type().Elem())
		ptr.Elem().Set(elem)
		target.Set(ptr)
		return nil
	}

	if target.CanAddr() {
		if scanner, ok := target.Addr().Interface().(sql.Scanner); ok {
			return scanner.Scan(value)
		}
	}

	src := reflect.ValueOf(value)
	if src.IsValid() && src.Type().AssignableTo(target.Type()) {
		target.Set(src)
		return nil
	}
	if src.IsValid() && src.Type().ConvertibleTo(target.Type()) {
		target.Set(src.Convert(target.Type()))
		return nil
	}

	switch target.Kind() {
	case reflect.String:
		switch v := value.(type) {
		case []byte:
			target.SetString(string(v))
		default:
			target.SetString(fmt.Sprintf("%v", value))
		}
		return nil
	case reflect.Bool:
		v, err := toBool(value)
		if err != nil {
			return err
		}
		target.SetBool(v)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, err := toInt64(value)
		if err != nil {
			return err
		}
		target.SetInt(v)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v, err := toUint64(value)
		if err != nil {
			return err
		}
		target.SetUint(v)
		return nil
	case reflect.Float32, reflect.Float64:
		v, err := toFloat64(value)
		if err != nil {
			return err
		}
		target.SetFloat(v)
		return nil
	case reflect.Interface:
		target.Set(reflect.ValueOf(value))
		return nil
	case reflect.Slice:
		if target.Type().Elem().Kind() == reflect.Uint8 {
			if v, ok := value.([]byte); ok {
				target.SetBytes(v)
				return nil
			}
		}
	}

	return sessionErrorf("cannot assign %T to %s", value, target.Type())
}

func toInt64(value any) (int64, error) {
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case uint:
		return int64(v), nil
	case uint8:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case uint64:
		return int64(v), nil
	case float32:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case []byte:
		return strconv.ParseInt(string(v), 10, 64)
	case string:
		return strconv.ParseInt(v, 10, 64)
	default:
		return 0, sessionErrorf("cannot convert %T to int64", value)
	}
}

func toUint64(value any) (uint64, error) {
	switch v := value.(type) {
	case int:
		return uint64(v), nil
	case int8:
		return uint64(v), nil
	case int16:
		return uint64(v), nil
	case int32:
		return uint64(v), nil
	case int64:
		return uint64(v), nil
	case uint:
		return uint64(v), nil
	case uint8:
		return uint64(v), nil
	case uint16:
		return uint64(v), nil
	case uint32:
		return uint64(v), nil
	case uint64:
		return v, nil
	case float32:
		return uint64(v), nil
	case float64:
		return uint64(v), nil
	case []byte:
		return strconv.ParseUint(string(v), 10, 64)
	case string:
		return strconv.ParseUint(v, 10, 64)
	default:
		return 0, sessionErrorf("cannot convert %T to uint64", value)
	}
}

func toFloat64(value any) (float64, error) {
	switch v := value.(type) {
	case int:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint8:
		return float64(v), nil
	case uint16:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case float32:
		return float64(v), nil
	case float64:
		return v, nil
	case []byte:
		return strconv.ParseFloat(string(v), 64)
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, sessionErrorf("cannot convert %T to float64", value)
	}
}

func toBool(value any) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case int64:
		return v != 0, nil
	case int:
		return v != 0, nil
	case []byte:
		return strconv.ParseBool(string(v))
	case string:
		return strconv.ParseBool(v)
	default:
		return false, sessionErrorf("cannot convert %T to bool", value)
	}
}

func toSnakeCase(value string) string {
	if value == "" {
		return ""
	}

	var b strings.Builder
	for i, r := range value {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}
