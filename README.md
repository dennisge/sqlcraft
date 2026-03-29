# sqlcraft

A MyBatis-style fluent SQL builder for Go with pluggable execution providers.

[![Go Reference](https://pkg.go.dev/badge/github.com/dennisge/sqlcraft.svg)](https://pkg.go.dev/github.com/dennisge/sqlcraft)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

## Features

- **Fluent API** — chainable methods for SELECT, INSERT, UPDATE, DELETE
- **Provider-aware binding** — `#{name}` placeholders are compiled to the active provider safely
- **Literal injection** — `${name}` placeholders for trusted column/table names
- **Selective methods** — `WhereSelective`, `SetSelective`, `ValuesSelective` skip zero-valued args automatically
- **WhereIn / WhereNotIn** — convenient IN clause builders with type-safe int64 variants
- **Transaction support** — simple callback-based transactions
- **Pluggable providers** — choose [GORM](https://gorm.io) or native `database/sql`
- **Multi-database** — MySQL and PostgreSQL helpers for both providers
- **Generated ID APIs** — `ExecResult()` for insert metadata and `Returning(...)` for PostgreSQL-style returned rows
- **Three layers** — use only what you need: SQL text assembly, fluent session, or driver setup

## Installation

```bash
go get github.com/dennisge/sqlcraft
```

## Quick Start

### 1. Choose a provider

`sqlcraft` separates database connection helpers from the fluent session API:

- `driver/mysql` and `driver/postgres` open either GORM or `database/sql` connections
- `session.NewGorm(...)` / `session.New(...)` bind a GORM connection to the fluent API
- `session.NewStdMySQL(...)`, `session.NewStdPostgres(...)`, or `session.NewStd(...)` bind a native `database/sql` connection

### 2. Connect with GORM provider

```go
import (
    "github.com/dennisge/sqlcraft/driver"
    "github.com/dennisge/sqlcraft/driver/mysql"
    "github.com/dennisge/sqlcraft/session"
)

db, err := mysql.OpenGorm(&driver.Config{
    DSN:     "user:pass@tcp(127.0.0.1:3306)/mydb?charset=utf8mb4&parseTime=True&loc=UTC",
    MaxOpen: 25,
    MaxIdle: 10,
})
```

```go
err := session.NewGorm(db).
    Select("id", "user_name", "status").
    From("users").
    Where("status = #{status}", 1).
    Scan(&users)
```

### 3. Connect with native `database/sql`

```go
import (
    "github.com/dennisge/sqlcraft/driver"
    "github.com/dennisge/sqlcraft/driver/postgres"
    "github.com/dennisge/sqlcraft/session"
)

db, err := postgres.OpenStd(&driver.Config{
    DSN:     "postgres://user:pass@127.0.0.1:5432/mydb?sslmode=disable",
    MaxOpen: 25,
    MaxIdle: 10,
})
```

```go
err := session.NewStdPostgres(db).
    Select("id", "user_name", "status").
    From("users").
    Where("status = #{status}", 1).
    Scan(&users)
```

### 4. Query

The following examples use the GORM provider for brevity. For `database/sql`, keep the chain the same and swap the constructor to `session.NewStdMySQL(...)`, `session.NewStdPostgres(...)`, or `session.NewStd(...)`.

```go
type User struct {
    ID       int64
    UserName string
    Status   int
}

var users []User
err := session.NewGorm(db).
    Select("id", "user_name", "status").
    From("users u").
    Where("status = #{status}", 1).
    WhereSelective("user_name LIKE #{name}", name). // skipped if name is ""
    OrderBy("id DESC").
    Limit(10).
    Scan(&users)
```

### 5. Insert

```go
rows, err := session.NewGorm(db).
    InsertInto("users").
    Values("user_name", "alice").
    Values("status", 1).
    Exec()
```

### 6. Bulk Insert

```go
rows, err := session.NewGorm(db).
    InsertInto("users").
    IntoColumns("user_name", "status").
    IntoMultiValues([][]any{
        {"alice", 1},
        {"bob", 1},
        {"charlie", 0},
    }).
    Exec()
```

### 7. Update

```go
rows, err := session.NewGorm(db).
    Update("users").
    Set("status", 0).
    SetSelective("user_name", newName). // skipped if newName is ""
    Where("id = #{id}", 42).
    Exec()
```

### 8. Delete

```go
rows, err := session.NewGorm(db).
    DeleteFrom("users").
    Where("status = #{status}", 0).
    Exec()
```

### 9. Transaction

```go
err := session.NewGorm(db).Transaction(func(tx session.Session) error {
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

### 10. Dynamic column injection

Use `${...}` for **trusted** column/table names only (never user input):

```go
var results []Map
err := session.NewGorm(db).
    Select("${col}", "count(*)").
    From("orders").
    GroupBy("${col}").
    AddParam("${col}", "region").
    Scan(&results)
```

### 11. Insert metadata and generated IDs

Use `ExecResult()` when the provider/database can expose `LastInsertID` style metadata:

```go
result, err := session.NewStdMySQL(db).
    InsertInto("users").
    Values("user_name", "alice").
    ExecResult()

firstID, err := result.InsertID()
batchIDs, err := result.InsertIDs() // derives [firstID, firstID+1, ...] for sequential auto-increment inserts
```

For databases like PostgreSQL, prefer `Returning(...)` and `Scan(...)`:

```go
type InsertedID struct {
    ID int64 `db:"id"`
}

var inserted []InsertedID
err := session.NewStdPostgres(db).
    InsertInto("users").
    IntoColumns("user_name", "status").
    IntoMultiValues([][]any{
        {"alice", 1},
        {"bob", 1},
    }).
    Returning("id").
    Scan(&inserted)
```

## Placeholder Reference

| Syntax | Behavior | Safety |
|--------|----------|--------|
| `#{name}` | Parameterized binding; rendered per provider (`@name`, `?`, `$1`, ...) | Safe against SQL injection |
| `${name}` | Literal string replacement | **Unsafe** — use only for trusted values |

## Architecture

```
sqlcraft/
  sqltext/    — low-level SQL text builder (no DB, no params)
  session/    — fluent Session with provider abstraction (GORM or database/sql)
  driver/     — connection helpers
    mysql/    — MySQL helpers for GORM and database/sql
    postgres/ — PostgreSQL helpers for GORM and database/sql
```

## Low-level: sqltext only

If you only need SQL text assembly without parameter binding:

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

Apache License 2.0 — see [LICENSE](LICENSE) for details.
