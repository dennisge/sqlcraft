package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"

	sqldriver "github.com/dennisge/sqlcraft/driver"
	mysqlhelper "github.com/dennisge/sqlcraft/driver/mysql"
	postgreshelper "github.com/dennisge/sqlcraft/driver/postgres"
	"github.com/dennisge/sqlcraft/session"
	"gorm.io/gorm"
)

var errForceRollback = errors.New("force rollback")

func main() {
	log.SetFlags(0)

	mysqlCfg := &sqldriver.Config{
		DSN:     "root:root@tcp(127.0.0.1:3307)/sqlcraft?charset=utf8mb4&parseTime=True&loc=UTC",
		MaxOpen: 5,
		MaxIdle: 5,
	}
	pgCfg := &sqldriver.Config{
		DSN:     "postgres://postgres:postgres@127.0.0.1:5433/sqlcraft?sslmode=disable",
		MaxOpen: 5,
		MaxIdle: 5,
	}

	if err := runMySQL(mysqlCfg); err != nil {
		log.Fatal(err)
	}
	if err := runPostgres(pgCfg); err != nil {
		log.Fatal(err)
	}
}

func runMySQL(cfg *sqldriver.Config) error {
	stdDB, err := mysqlhelper.OpenStd(cfg)
	if err != nil {
		return fmt.Errorf("mysql open std: %w", err)
	}
	defer sqldriver.CloseSQL(stdDB)

	gormDB, err := mysqlhelper.OpenGorm(cfg)
	if err != nil {
		return fmt.Errorf("mysql open gorm: %w", err)
	}
	defer sqldriver.Close(gormDB)

	if err := recreateMySQLSchema(stdDB); err != nil {
		return fmt.Errorf("mysql schema: %w", err)
	}

	fmt.Println("== MySQL ==")

	stdSingleMarker := "mysql-std-single"
	stdSingleResult, err := mysqlhelper.NewSession(stdDB).
		InsertInto("exec_probe").
		Values("marker", stdSingleMarker).
		Values("note", "std single exec").
		ExecResult()
	if err != nil {
		return fmt.Errorf("mysql std single exec: %w", err)
	}
	stdSingleIDs, err := fetchIDs(stdDB, "SELECT id FROM exec_probe WHERE marker = ? ORDER BY id", stdSingleMarker)
	if err != nil {
		return err
	}
	stdSingleInsertID, _ := stdSingleResult.InsertID()
	fmt.Printf("std Session.ExecResult single  -> rowsAffected=%d, lastInsertID=%d, insertedIDs=%v\n", stdSingleResult.RowsAffected, stdSingleInsertID, stdSingleIDs)

	stdBatchPrefix := "mysql-std-batch"
	stdBatchResult, err := mysqlhelper.NewSession(stdDB).
		InsertInto("exec_probe").
		IntoColumns("marker", "note").
		IntoMultiValues([][]any{
			{stdBatchPrefix + "-1", "std batch 1"},
			{stdBatchPrefix + "-2", "std batch 2"},
			{stdBatchPrefix + "-3", "std batch 3"},
		}).
		ExecResult()
	if err != nil {
		return fmt.Errorf("mysql std batch exec: %w", err)
	}
	stdBatchIDs, err := fetchIDs(stdDB, "SELECT id FROM exec_probe WHERE marker LIKE ? ORDER BY id", stdBatchPrefix+"-%")
	if err != nil {
		return err
	}
	derivedStdBatchIDs, _ := stdBatchResult.InsertIDs()
	fmt.Printf("std Session.ExecResult batch   -> rowsAffected=%d, firstInsertID=%d, derivedIDs=%v, actualInsertedIDs=%v\n", stdBatchResult.RowsAffected, stdBatchResult.LastInsertID, derivedStdBatchIDs, stdBatchIDs)

	rawSingleResult, err := stdDB.Exec(
		"INSERT INTO exec_probe (marker, note) VALUES (?, ?)",
		"mysql-raw-single",
		"direct sql.Result single",
	)
	if err != nil {
		return fmt.Errorf("mysql raw single exec: %w", err)
	}
	rawSingleID, err := rawSingleResult.LastInsertId()
	if err != nil {
		return fmt.Errorf("mysql raw single last insert id: %w", err)
	}
	fmt.Printf("direct sql.Result single       -> LastInsertId=%d\n", rawSingleID)

	rawBatchResult, err := stdDB.Exec(
		"INSERT INTO exec_probe (marker, note) VALUES (?, ?), (?, ?), (?, ?)",
		"mysql-raw-batch-1", "direct batch 1",
		"mysql-raw-batch-2", "direct batch 2",
		"mysql-raw-batch-3", "direct batch 3",
	)
	if err != nil {
		return fmt.Errorf("mysql raw batch exec: %w", err)
	}
	rawBatchFirstID, err := rawBatchResult.LastInsertId()
	if err != nil {
		return fmt.Errorf("mysql raw batch last insert id: %w", err)
	}
	rawBatchIDs, err := fetchIDs(stdDB, "SELECT id FROM exec_probe WHERE marker LIKE ? ORDER BY id", "mysql-raw-batch-%")
	if err != nil {
		return err
	}
	fmt.Printf("direct sql.Result batch        -> LastInsertId=%d, actualInsertedIDs=%v\n", rawBatchFirstID, rawBatchIDs)

	gormSingleMarker := "mysql-gorm-single"
	gormSingleResult, err := mysqlhelper.NewGormSession(gormDB).
		InsertInto("exec_probe").
		Values("marker", gormSingleMarker).
		Values("note", "gorm single exec").
		ExecResult()
	if err != nil {
		return fmt.Errorf("mysql gorm single exec: %w", err)
	}
	gormSingleIDs, err := fetchIDs(stdDB, "SELECT id FROM exec_probe WHERE marker = ? ORDER BY id", gormSingleMarker)
	if err != nil {
		return err
	}
	gormSingleInsertID, _ := gormSingleResult.InsertID()
	fmt.Printf("gorm Session.ExecResult single -> rowsAffected=%d, lastInsertID=%d, insertedIDs=%v\n", gormSingleResult.RowsAffected, gormSingleInsertID, gormSingleIDs)

	gormBatchPrefix := "mysql-gorm-batch"
	gormBatchResult, err := mysqlhelper.NewGormSession(gormDB).
		InsertInto("exec_probe").
		IntoColumns("marker", "note").
		IntoMultiValues([][]any{
			{gormBatchPrefix + "-1", "gorm batch 1"},
			{gormBatchPrefix + "-2", "gorm batch 2"},
			{gormBatchPrefix + "-3", "gorm batch 3"},
		}).
		ExecResult()
	if err != nil {
		return fmt.Errorf("mysql gorm batch exec: %w", err)
	}
	gormBatchIDs, err := fetchIDs(stdDB, "SELECT id FROM exec_probe WHERE marker LIKE ? ORDER BY id", gormBatchPrefix+"-%")
	if err != nil {
		return err
	}
	derivedGormBatchIDs, _ := gormBatchResult.InsertIDs()
	fmt.Printf("gorm Session.ExecResult batch  -> rowsAffected=%d, firstInsertID=%d, derivedIDs=%v, actualInsertedIDs=%v\n", gormBatchResult.RowsAffected, gormBatchResult.LastInsertID, derivedGormBatchIDs, gormBatchIDs)

	if err := runMySQLTransactions(stdDB, gormDB); err != nil {
		return err
	}

	return nil
}

func runPostgres(cfg *sqldriver.Config) error {
	stdDB, err := postgreshelper.OpenStd(cfg)
	if err != nil {
		return fmt.Errorf("postgres open std: %w", err)
	}
	defer sqldriver.CloseSQL(stdDB)

	gormDB, err := postgreshelper.OpenGorm(cfg)
	if err != nil {
		return fmt.Errorf("postgres open gorm: %w", err)
	}
	defer sqldriver.Close(gormDB)

	if err := recreatePostgresSchema(stdDB); err != nil {
		return fmt.Errorf("postgres schema: %w", err)
	}

	fmt.Println()
	fmt.Println("== PostgreSQL ==")

	stdSingleMarker := "pg-std-single"
	stdSingleResult, err := postgreshelper.NewSession(stdDB).
		InsertInto("exec_probe").
		Values("marker", stdSingleMarker).
		Values("note", "std single exec").
		ExecResult()
	if err != nil {
		return fmt.Errorf("postgres std single exec: %w", err)
	}
	stdSingleIDs, err := fetchIDs(stdDB, "SELECT id FROM exec_probe WHERE marker = $1 ORDER BY id", stdSingleMarker)
	if err != nil {
		return err
	}
	_, stdSingleInsertErr := stdSingleResult.InsertID()
	fmt.Printf("std Session.ExecResult single  -> rowsAffected=%d, insertIDErr=%v, insertedIDs=%v\n", stdSingleResult.RowsAffected, stdSingleInsertErr, stdSingleIDs)

	stdBatchPrefix := "pg-std-batch"
	stdBatchResult, err := postgreshelper.NewSession(stdDB).
		InsertInto("exec_probe").
		IntoColumns("marker", "note").
		IntoMultiValues([][]any{
			{stdBatchPrefix + "-1", "std batch 1"},
			{stdBatchPrefix + "-2", "std batch 2"},
			{stdBatchPrefix + "-3", "std batch 3"},
		}).
		ExecResult()
	if err != nil {
		return fmt.Errorf("postgres std batch exec: %w", err)
	}
	stdBatchIDs, err := fetchIDs(stdDB, "SELECT id FROM exec_probe WHERE marker LIKE $1 ORDER BY id", stdBatchPrefix+"-%")
	if err != nil {
		return err
	}
	_, stdBatchInsertErr := stdBatchResult.InsertIDs()
	fmt.Printf("std Session.ExecResult batch   -> rowsAffected=%d, insertIDsErr=%v, actualInsertedIDs=%v\n", stdBatchResult.RowsAffected, stdBatchInsertErr, stdBatchIDs)

	rawSingleResult, err := stdDB.Exec(
		"INSERT INTO exec_probe (marker, note) VALUES ($1, $2)",
		"pg-raw-single",
		"direct sql.Result single",
	)
	if err != nil {
		return fmt.Errorf("postgres raw single exec: %w", err)
	}
	rawSingleID, lastInsertErr := rawSingleResult.LastInsertId()
	fmt.Printf("direct sql.Result single       -> LastInsertId=%d, err=%v\n", rawSingleID, lastInsertErr)

	rawBatchResult, err := stdDB.Exec(
		"INSERT INTO exec_probe (marker, note) VALUES ($1, $2), ($3, $4), ($5, $6)",
		"pg-raw-batch-1", "direct batch 1",
		"pg-raw-batch-2", "direct batch 2",
		"pg-raw-batch-3", "direct batch 3",
	)
	if err != nil {
		return fmt.Errorf("postgres raw batch exec: %w", err)
	}
	rawBatchFirstID, batchLastInsertErr := rawBatchResult.LastInsertId()
	rawBatchIDs, err := fetchIDs(stdDB, "SELECT id FROM exec_probe WHERE marker LIKE $1 ORDER BY id", "pg-raw-batch-%")
	if err != nil {
		return err
	}
	fmt.Printf("direct sql.Result batch        -> LastInsertId=%d, err=%v, actualInsertedIDs=%v\n", rawBatchFirstID, batchLastInsertErr, rawBatchIDs)

	var returningSingleStd []idRow
	err = postgreshelper.NewSession(stdDB).
		InsertInto("exec_probe").
		Values("marker", "pg-std-returning-single").
		Values("note", "std returning single").
		Returning("id").
		Scan(&returningSingleStd)
	if err != nil {
		return fmt.Errorf("postgres std returning single: %w", err)
	}
	fmt.Printf("std INSERT ... RETURNING       -> returnedIDs=%v\n", extractIDs(returningSingleStd))

	var returningBatchStd []idRow
	err = postgreshelper.NewSession(stdDB).
		InsertInto("exec_probe").
		IntoColumns("marker", "note").
		IntoMultiValues([][]any{
			{"pg-std-returning-batch-1", "std returning batch 1"},
			{"pg-std-returning-batch-2", "std returning batch 2"},
			{"pg-std-returning-batch-3", "std returning batch 3"},
		}).
		Returning("id").
		Scan(&returningBatchStd)
	if err != nil {
		return fmt.Errorf("postgres std returning batch: %w", err)
	}
	fmt.Printf("std batch ... RETURNING        -> returnedIDs=%v\n", extractIDs(returningBatchStd))

	var returningSingleGorm []idRow
	err = postgreshelper.NewGormSession(gormDB).
		InsertInto("exec_probe").
		Values("marker", "pg-gorm-returning-single").
		Values("note", "gorm returning single").
		Returning("id").
		Scan(&returningSingleGorm)
	if err != nil {
		return fmt.Errorf("postgres gorm returning single: %w", err)
	}
	fmt.Printf("gorm INSERT ... RETURNING      -> returnedIDs=%v\n", extractIDs(returningSingleGorm))

	var returningBatchGorm []idRow
	err = postgreshelper.NewGormSession(gormDB).
		InsertInto("exec_probe").
		IntoColumns("marker", "note").
		IntoMultiValues([][]any{
			{"pg-gorm-returning-batch-1", "gorm returning batch 1"},
			{"pg-gorm-returning-batch-2", "gorm returning batch 2"},
			{"pg-gorm-returning-batch-3", "gorm returning batch 3"},
		}).
		Returning("id").
		Scan(&returningBatchGorm)
	if err != nil {
		return fmt.Errorf("postgres gorm returning batch: %w", err)
	}
	fmt.Printf("gorm batch ... RETURNING       -> returnedIDs=%v\n", extractIDs(returningBatchGorm))

	if err := runPostgresTransactions(stdDB, gormDB); err != nil {
		return err
	}

	return nil
}

type idRow struct {
	ID int64 `db:"id"`
}

func extractIDs(rows []idRow) []int64 {
	out := make([]int64, len(rows))
	for i, row := range rows {
		out[i] = row.ID
	}
	return out
}

func recreateMySQLSchema(db *sql.DB) error {
	stmts := []string{
		"DROP TABLE IF EXISTS exec_probe",
		`CREATE TABLE exec_probe (
			id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			marker VARCHAR(128) NOT NULL,
			note VARCHAR(255) NOT NULL
		)`,
	}
	return execAll(db, stmts)
}

func recreatePostgresSchema(db *sql.DB) error {
	stmts := []string{
		"DROP TABLE IF EXISTS exec_probe",
		`CREATE TABLE exec_probe (
			id BIGSERIAL PRIMARY KEY,
			marker VARCHAR(128) NOT NULL,
			note VARCHAR(255) NOT NULL
		)`,
	}
	return execAll(db, stmts)
}

func execAll(db *sql.DB, stmts []string) error {
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("%s: %w", compactSQL(stmt), err)
		}
	}
	return nil
}

func runMySQLTransactions(stdDB *sql.DB, gormDB *gorm.DB) error {
	if err := mysqlhelper.NewSession(stdDB).Transaction(func(tx session.Session) error {
		_, err := tx.InsertInto("exec_probe").Values("marker", "mysql-std-tx-commit").Values("note", "std tx commit").Exec()
		return err
	}); err != nil {
		return fmt.Errorf("mysql std tx commit: %w", err)
	}
	if err := expectCount(stdDB, "SELECT COUNT(*) FROM exec_probe WHERE marker = ?", "mysql-std-tx-commit", 1); err != nil {
		return fmt.Errorf("mysql std tx commit verify: %w", err)
	}

	rollbackErr := mysqlhelper.NewSession(stdDB).Transaction(func(tx session.Session) error {
		if _, err := tx.InsertInto("exec_probe").Values("marker", "mysql-std-tx-rollback").Values("note", "std tx rollback").Exec(); err != nil {
			return err
		}
		return errForceRollback
	})
	if !errors.Is(rollbackErr, errForceRollback) {
		return fmt.Errorf("mysql std tx rollback: want %v, got %v", errForceRollback, rollbackErr)
	}
	if err := expectCount(stdDB, "SELECT COUNT(*) FROM exec_probe WHERE marker = ?", "mysql-std-tx-rollback", 0); err != nil {
		return fmt.Errorf("mysql std tx rollback verify: %w", err)
	}

	if err := mysqlhelper.NewGormSession(gormDB).Transaction(func(tx session.Session) error {
		_, err := tx.InsertInto("exec_probe").Values("marker", "mysql-gorm-tx-commit").Values("note", "gorm tx commit").Exec()
		return err
	}); err != nil {
		return fmt.Errorf("mysql gorm tx commit: %w", err)
	}
	if err := expectCount(stdDB, "SELECT COUNT(*) FROM exec_probe WHERE marker = ?", "mysql-gorm-tx-commit", 1); err != nil {
		return fmt.Errorf("mysql gorm tx commit verify: %w", err)
	}

	rollbackErr = mysqlhelper.NewGormSession(gormDB).Transaction(func(tx session.Session) error {
		if _, err := tx.InsertInto("exec_probe").Values("marker", "mysql-gorm-tx-rollback").Values("note", "gorm tx rollback").Exec(); err != nil {
			return err
		}
		return errForceRollback
	})
	if !errors.Is(rollbackErr, errForceRollback) {
		return fmt.Errorf("mysql gorm tx rollback: want %v, got %v", errForceRollback, rollbackErr)
	}
	if err := expectCount(stdDB, "SELECT COUNT(*) FROM exec_probe WHERE marker = ?", "mysql-gorm-tx-rollback", 0); err != nil {
		return fmt.Errorf("mysql gorm tx rollback verify: %w", err)
	}

	if err := gormDB.Transaction(func(gtx *gorm.DB) error {
		_, err := mysqlhelper.NewGormSession(gtx).
			InsertInto("exec_probe").
			Values("marker", "mysql-gorm-provider-commit").
			Values("note", "gorm provider tx commit").
			Exec()
		return err
	}); err != nil {
		return fmt.Errorf("mysql gorm provider tx commit: %w", err)
	}
	if err := expectCount(stdDB, "SELECT COUNT(*) FROM exec_probe WHERE marker = ?", "mysql-gorm-provider-commit", 1); err != nil {
		return fmt.Errorf("mysql gorm provider tx commit verify: %w", err)
	}

	rollbackErr = gormDB.Transaction(func(gtx *gorm.DB) error {
		if _, err := mysqlhelper.NewGormSession(gtx).
			InsertInto("exec_probe").
			Values("marker", "mysql-gorm-provider-rollback").
			Values("note", "gorm provider tx rollback").
			Exec(); err != nil {
			return err
		}
		return errForceRollback
	})
	if !errors.Is(rollbackErr, errForceRollback) {
		return fmt.Errorf("mysql gorm provider tx rollback: want %v, got %v", errForceRollback, rollbackErr)
	}
	if err := expectCount(stdDB, "SELECT COUNT(*) FROM exec_probe WHERE marker = ?", "mysql-gorm-provider-rollback", 0); err != nil {
		return fmt.Errorf("mysql gorm provider tx rollback verify: %w", err)
	}

	sqlTx, err := stdDB.Begin()
	if err != nil {
		return fmt.Errorf("mysql std provider tx begin: %w", err)
	}
	if _, err := mysqlhelper.NewTxSession(sqlTx).
		InsertInto("exec_probe").
		Values("marker", "mysql-std-provider-commit").
		Values("note", "std provider tx commit").
		Exec(); err != nil {
		_ = sqlTx.Rollback()
		return fmt.Errorf("mysql std provider tx exec: %w", err)
	}
	if err := sqlTx.Commit(); err != nil {
		return fmt.Errorf("mysql std provider tx commit: %w", err)
	}
	if err := expectCount(stdDB, "SELECT COUNT(*) FROM exec_probe WHERE marker = ?", "mysql-std-provider-commit", 1); err != nil {
		return fmt.Errorf("mysql std provider tx commit verify: %w", err)
	}

	sqlTx, err = stdDB.Begin()
	if err != nil {
		return fmt.Errorf("mysql std provider tx rollback begin: %w", err)
	}
	if _, err := mysqlhelper.NewTxSession(sqlTx).
		InsertInto("exec_probe").
		Values("marker", "mysql-std-provider-rollback").
		Values("note", "std provider tx rollback").
		Exec(); err != nil {
		_ = sqlTx.Rollback()
		return fmt.Errorf("mysql std provider tx rollback exec: %w", err)
	}
	if err := sqlTx.Rollback(); err != nil {
		return fmt.Errorf("mysql std provider tx rollback: %w", err)
	}
	if err := expectCount(stdDB, "SELECT COUNT(*) FROM exec_probe WHERE marker = ?", "mysql-std-provider-rollback", 0); err != nil {
		return fmt.Errorf("mysql std provider tx rollback verify: %w", err)
	}

	fmt.Println("transactions                     -> std/gorm commit+rollback verified")
	return nil
}

func runPostgresTransactions(stdDB *sql.DB, gormDB *gorm.DB) error {
	if err := postgreshelper.NewSession(stdDB).Transaction(func(tx session.Session) error {
		_, err := tx.InsertInto("exec_probe").Values("marker", "pg-std-tx-commit").Values("note", "std tx commit").Exec()
		return err
	}); err != nil {
		return fmt.Errorf("postgres std tx commit: %w", err)
	}
	if err := expectCount(stdDB, "SELECT COUNT(*) FROM exec_probe WHERE marker = $1", "pg-std-tx-commit", 1); err != nil {
		return fmt.Errorf("postgres std tx commit verify: %w", err)
	}

	rollbackErr := postgreshelper.NewSession(stdDB).Transaction(func(tx session.Session) error {
		if _, err := tx.InsertInto("exec_probe").Values("marker", "pg-std-tx-rollback").Values("note", "std tx rollback").Exec(); err != nil {
			return err
		}
		return errForceRollback
	})
	if !errors.Is(rollbackErr, errForceRollback) {
		return fmt.Errorf("postgres std tx rollback: want %v, got %v", errForceRollback, rollbackErr)
	}
	if err := expectCount(stdDB, "SELECT COUNT(*) FROM exec_probe WHERE marker = $1", "pg-std-tx-rollback", 0); err != nil {
		return fmt.Errorf("postgres std tx rollback verify: %w", err)
	}

	if err := postgreshelper.NewGormSession(gormDB).Transaction(func(tx session.Session) error {
		_, err := tx.InsertInto("exec_probe").Values("marker", "pg-gorm-tx-commit").Values("note", "gorm tx commit").Exec()
		return err
	}); err != nil {
		return fmt.Errorf("postgres gorm tx commit: %w", err)
	}
	if err := expectCount(stdDB, "SELECT COUNT(*) FROM exec_probe WHERE marker = $1", "pg-gorm-tx-commit", 1); err != nil {
		return fmt.Errorf("postgres gorm tx commit verify: %w", err)
	}

	rollbackErr = postgreshelper.NewGormSession(gormDB).Transaction(func(tx session.Session) error {
		if _, err := tx.InsertInto("exec_probe").Values("marker", "pg-gorm-tx-rollback").Values("note", "gorm tx rollback").Exec(); err != nil {
			return err
		}
		return errForceRollback
	})
	if !errors.Is(rollbackErr, errForceRollback) {
		return fmt.Errorf("postgres gorm tx rollback: want %v, got %v", errForceRollback, rollbackErr)
	}
	if err := expectCount(stdDB, "SELECT COUNT(*) FROM exec_probe WHERE marker = $1", "pg-gorm-tx-rollback", 0); err != nil {
		return fmt.Errorf("postgres gorm tx rollback verify: %w", err)
	}

	if err := gormDB.Transaction(func(gtx *gorm.DB) error {
		_, err := postgreshelper.NewGormSession(gtx).
			InsertInto("exec_probe").
			Values("marker", "pg-gorm-provider-commit").
			Values("note", "gorm provider tx commit").
			Exec()
		return err
	}); err != nil {
		return fmt.Errorf("postgres gorm provider tx commit: %w", err)
	}
	if err := expectCount(stdDB, "SELECT COUNT(*) FROM exec_probe WHERE marker = $1", "pg-gorm-provider-commit", 1); err != nil {
		return fmt.Errorf("postgres gorm provider tx commit verify: %w", err)
	}

	rollbackErr = gormDB.Transaction(func(gtx *gorm.DB) error {
		if _, err := postgreshelper.NewGormSession(gtx).
			InsertInto("exec_probe").
			Values("marker", "pg-gorm-provider-rollback").
			Values("note", "gorm provider tx rollback").
			Exec(); err != nil {
			return err
		}
		return errForceRollback
	})
	if !errors.Is(rollbackErr, errForceRollback) {
		return fmt.Errorf("postgres gorm provider tx rollback: want %v, got %v", errForceRollback, rollbackErr)
	}
	if err := expectCount(stdDB, "SELECT COUNT(*) FROM exec_probe WHERE marker = $1", "pg-gorm-provider-rollback", 0); err != nil {
		return fmt.Errorf("postgres gorm provider tx rollback verify: %w", err)
	}

	sqlTx, err := stdDB.Begin()
	if err != nil {
		return fmt.Errorf("postgres std provider tx begin: %w", err)
	}
	if _, err := postgreshelper.NewTxSession(sqlTx).
		InsertInto("exec_probe").
		Values("marker", "pg-std-provider-commit").
		Values("note", "std provider tx commit").
		Exec(); err != nil {
		_ = sqlTx.Rollback()
		return fmt.Errorf("postgres std provider tx exec: %w", err)
	}
	if err := sqlTx.Commit(); err != nil {
		return fmt.Errorf("postgres std provider tx commit: %w", err)
	}
	if err := expectCount(stdDB, "SELECT COUNT(*) FROM exec_probe WHERE marker = $1", "pg-std-provider-commit", 1); err != nil {
		return fmt.Errorf("postgres std provider tx commit verify: %w", err)
	}

	sqlTx, err = stdDB.Begin()
	if err != nil {
		return fmt.Errorf("postgres std provider tx rollback begin: %w", err)
	}
	if _, err := postgreshelper.NewTxSession(sqlTx).
		InsertInto("exec_probe").
		Values("marker", "pg-std-provider-rollback").
		Values("note", "std provider tx rollback").
		Exec(); err != nil {
		_ = sqlTx.Rollback()
		return fmt.Errorf("postgres std provider tx rollback exec: %w", err)
	}
	if err := sqlTx.Rollback(); err != nil {
		return fmt.Errorf("postgres std provider tx rollback: %w", err)
	}
	if err := expectCount(stdDB, "SELECT COUNT(*) FROM exec_probe WHERE marker = $1", "pg-std-provider-rollback", 0); err != nil {
		return fmt.Errorf("postgres std provider tx rollback verify: %w", err)
	}

	fmt.Println("transactions                     -> std/gorm commit+rollback verified")
	return nil
}

func fetchIDs(db *sql.DB, query string, arg any) ([]int64, error) {
	rows, err := db.Query(query, arg)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", compactSQL(query), err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	slices.Sort(ids)
	return ids, nil
}

func expectCount(db *sql.DB, query string, arg any, want int64) error {
	got, err := fetchCount(db, query, arg)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("count = %d, want %d", got, want)
	}
	return nil
}

func fetchCount(db *sql.DB, query string, arg any) (int64, error) {
	row := db.QueryRow(query, arg)
	var count int64
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("%s: %w", compactSQL(query), err)
	}
	return count, nil
}

func compactSQL(sqlText string) string {
	fields := strings.Fields(sqlText)
	if len(fields) == 0 {
		return sqlText
	}
	return strings.Join(fields, " ")
}
