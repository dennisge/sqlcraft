# sqlcraft

A MyBatis-style fluent SQL builder for Go with pluggable execution providers.

[![Go Reference](https://pkg.go.dev/badge/github.com/dennisge/sqlcraft.svg)](https://pkg.go.dev/github.com/dennisge/sqlcraft)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

## Features

- **Fluent API** for SELECT, INSERT, UPDATE, and DELETE
- **Provider-aware binding** for GORM and native `database/sql`
- **Named placeholders** with `#{name}` for safe parameter binding
- **Trusted literal injection** with `${name}` for column/table names
- **Selective methods** such as `WhereSelective`, `SetSelective`, and `ValuesSelective`
- **Convenient IN builders** with `WhereIn`, `WhereNotIn`, and `[]int64` helpers
- **Transaction support** with a callback-based API
- **Generated ID helpers** via `ExecResult()` and `Returning(...)`
- **Layered design** with `sqltext`, `session`, and `driver` packages

## Installation

```bash
go get github.com/dennisge/sqlcraft
```

## Quick Start

### Pick the right entry point

For most application code, prefer the provider-specific helper packages in `driver/mysql` and `driver/postgres`.
They hide dialect details for `database/sql` and make the recommended path obvious.

| Situation | Recommended API | Why |
|----------|------------------|-----|
| You have a `driver.Config` and want a GORM-backed session | `mysql.OpenGormSession(cfg)` / `postgres.OpenGormSession(cfg)` | Opens the DB and returns a ready-to-use `session.Session` |
| You have a `driver.Config` and want a native `database/sql` session | `mysql.OpenSession(cfg)` / `postgres.OpenSession(cfg)` | Opens the DB, applies pool settings, and hides dialect wiring |
| You already have a `*gorm.DB` | `session.NewGorm(db)` | GORM already knows the database dialect |
| You already have a `*sql.DB` | `mysql.NewSession(db)` / `postgres.NewSession(db)` | Keeps dialect selection in the provider package |
| You need the raw DB handle for `Close`, `Ping`, or other integrations | `mysql.OpenGorm(cfg)` / `mysql.OpenStd(cfg)` and then bind manually | Gives you both the raw connection and the fluent session |
| You need advanced or custom wiring | `session.NewStd(db, session.Dialect...)` | Lowest-level escape hatch |

### GORM quick start

```go
import (
    "github.com/dennisge/sqlcraft/driver"
    "github.com/dennisge/sqlcraft/driver/mysql"
)

sess, err := mysql.OpenGormSession(&driver.Config{
    DSN:     "user:pass@tcp(127.0.0.1:3306)/mydb?charset=utf8mb4&parseTime=True&loc=UTC",
    MaxOpen: 25,
    MaxIdle: 10,
})
if err != nil {
    return err
}

type User struct {
    ID       int64
    UserName string
    Status   int
}

var users []User
err = sess.
    Select("id", "user_name", "status").
    From("users").
    Where("status = #{status}", 1).
    Scan(&users)
```

### `database/sql` quick start

```go
import (
    "github.com/dennisge/sqlcraft/driver"
    "github.com/dennisge/sqlcraft/driver/postgres"
)

sess, err := postgres.OpenSession(&driver.Config{
    DSN:     "postgres://user:pass@127.0.0.1:5432/mydb?sslmode=disable",
    MaxOpen: 25,
    MaxIdle: 10,
})
if err != nil {
    return err
}

type User struct {
    ID       int64
    UserName string
    Status   int
}

var users []User
err = sess.
    Select("id", "user_name", "status").
    From("users").
    Where("status = #{status}", 1).
    Scan(&users)
```

## Common Patterns

The following examples assume you already created `sess` using one of the snippets above.

### Query

```go
type User struct {
    ID       int64
    UserName string
    Status   int
}

var users []User
err := sess.
    Select("id", "user_name", "status").
    From("users u").
    Where("status = #{status}", 1).
    WhereSelective("user_name LIKE #{name}", name). // skipped if name is ""
    OrderBy("id DESC").
    Limit(10).
    Scan(&users)
```

### Insert

```go
rows, err := sess.
    InsertInto("users").
    Values("user_name", "alice").
    Values("status", 1).
    Exec()
```

### Bulk Insert

```go
rows, err := sess.
    InsertInto("users").
    IntoColumns("user_name", "status").
    IntoMultiValues([][]any{
        {"alice", 1},
        {"bob", 1},
        {"charlie", 0},
    }).
    Exec()
```

### Update

```go
rows, err := sess.
    Update("users").
    Set("status", 0).
    SetSelective("user_name", newName). // skipped if newName is ""
    Where("id = #{id}", 42).
    Exec()
```

### Delete

```go
rows, err := sess.
    DeleteFrom("users").
    Where("status = #{status}", 0).
    Exec()
```

### Transaction

```go
import "github.com/dennisge/sqlcraft/session"

err := sess.Transaction(func(tx session.Session) error {
    _, err := tx.Update("accounts").
        Set("balance", 100).
        Where("id = #{id}", 1).
        Exec()
    if err != nil {
        return err
    }

    _, err = tx.Update("accounts").
        Set("balance", 200).
        Where("id = #{id}", 2).
        Exec()
    return err
})
```

## Generated IDs

Generated ID handling is database-specific.

| Database | Recommended pattern | Notes |
|----------|---------------------|-------|
| MySQL | `ExecResult()` | `InsertID()` and `InsertIDs()` work for auto-increment inserts |
| PostgreSQL | `Returning(...).Scan(...)` | `LastInsertId` is not the standard path |

### MySQL-style insert metadata

```go
result, err := sess.
    InsertInto("users").
    Values("user_name", "alice").
    ExecResult()
if err != nil {
    return err
}

firstID, err := result.InsertID()
batchIDs, err := result.InsertIDs() // derives [firstID, firstID+1, ...]
```

### PostgreSQL-style `RETURNING`

```go
type InsertedID struct {
    ID int64 `db:"id"`
}

var inserted []InsertedID
err := sess.
    InsertInto("users").
    IntoColumns("user_name", "status").
    IntoMultiValues([][]any{
        {"alice", 1},
        {"bob", 1},
    }).
    Returning("id").
    Scan(&inserted)
```

## Dynamic Identifiers

Use `${...}` only for trusted column or table names.
Never pass user input through `${...}`.

```go
var results []map[string]any
err := sess.
    Select("${col}", "count(*) AS total").
    From("orders").
    GroupBy("${col}").
    AddParam("${col}", "region").
    Scan(&results)
```

## Placeholder Reference

| Syntax | Behavior | Safety |
|--------|----------|--------|
| `#{name}` | Parameterized binding; rendered per provider (`@name`, `?`, `$1`, ...) | Safe against SQL injection |
| `${name}` | Literal string replacement | Unsafe; use only for trusted values |

## Lower-Level APIs

If you need more control than the provider helpers offer:

- Use `mysql.OpenGorm(cfg)` / `mysql.OpenStd(cfg)` or `postgres.OpenGorm(cfg)` / `postgres.OpenStd(cfg)` when you need the raw DB handle
- Use `session.NewGorm(db)` when you already have a `*gorm.DB`
- Use `mysql.NewSession(db)` or `postgres.NewSession(db)` when you already have a `*sql.DB`
- Use `session.NewStd(db, session.DialectMySQL)` or `session.NewStd(db, session.DialectPostgres)` only for advanced manual wiring
- Use `sqltext` directly when you only want SQL string assembly

## Architecture

```text
sqlcraft/
  sqltext/    - low-level SQL text builder (no DB, no params)
  session/    - fluent Session with provider abstraction
  driver/     - connection and session helpers
    mysql/    - MySQL helpers for GORM and database/sql
    postgres/ - PostgreSQL helpers for GORM and database/sql
```

## `sqltext` Only

If you only need SQL text assembly without parameter binding or execution:

```go
import "github.com/dennisge/sqlcraft/sqltext"

sql := sqltext.New()
sql.Select("id", "name")
sql.From("users")
sql.Where("status = 1")
sql.OrderBy("id")
sql.Limit("10")

fmt.Println(sql.String())
// SELECT id, name
// FROM users
// WHERE (status = 1)
// ORDER BY id LIMIT 10
```

## License

Apache License 2.0 - see [LICENSE](LICENSE) for details.
