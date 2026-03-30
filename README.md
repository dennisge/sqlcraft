# sqlcraft

A MyBatis-style fluent SQL builder for Go with pluggable execution providers.

[![Go Reference](https://pkg.go.dev/badge/github.com/dennisge/sqlcraft.svg)](https://pkg.go.dev/github.com/dennisge/sqlcraft)
[![CI](https://github.com/dennisge/sqlcraft/actions/workflows/ci.yml/badge.svg?branch=master)](https://github.com/dennisge/sqlcraft/actions/workflows/ci.yml)
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

## Why sqlcraft

`sqlcraft` is aimed at teams that still want to write and read SQL directly,
but do not want to hand-roll placeholder binding, dialect differences, and
controller-level `if` trees for dynamic filters.

| Option | Strong at | Trade-off compared with `sqlcraft` |
|--------|-----------|-------------------------------------|
| GORM raw SQL | Great when the whole project already centers on GORM models and lifecycle hooks | Dynamic SQL and request-driven optional predicates are still largely manual |
| `sqlx` | Lightweight query execution and scanning with handwritten SQL | Leaves most dynamic SQL assembly, provider differences, and filter pruning to application code |
| `squirrel` / `goqu` | Builder-heavy composition and dialect-aware SQL generation | Their center of gravity is the builder DSL, not MyBatis-style named placeholders and API DTO-driven selective binding |
| `sqlcraft` | Readable handwritten SQL plus fluent composition, provider switching, and Selective APIs | Smaller ecosystem and narrower scope than larger query/ORM libraries |

Choose `sqlcraft` when you want:

- SQL to stay visible and reviewable in the codebase
- One fluent API that can sit on top of either GORM or `database/sql`
- Request DTOs to map naturally into optional SQL predicates
- A lighter-weight alternative to a full ORM, without falling back to raw string concatenation

## Quick Start

### Pick the right entry point

For most application code, prefer the provider-specific helper packages in `driver/mysql` and `driver/postgres`.
They hide dialect details for `database/sql` and make the recommended path obvious.

| Situation | Recommended API | Why |
|----------|------------------|-----|
| You want the fastest quick-start with GORM | `mysql.OpenGormSession(cfg)` / `postgres.OpenGormSession(cfg)` | Opens the DB and returns a ready-to-use `session.Session` |
| You want the fastest quick-start with native `database/sql` | `mysql.OpenSession(cfg)` / `postgres.OpenSession(cfg)` | Opens the DB, applies pool settings, and hides dialect wiring |
| You already have a `*gorm.DB` | `session.NewGorm(db)` | GORM already knows the database dialect |
| You already have a `*sql.DB` | `mysql.NewSession(db)` / `postgres.NewSession(db)` | Keeps dialect selection in the provider package |
| You are wiring a long-lived application service | `mysql.OpenGorm(cfg)` / `postgres.OpenGorm(cfg)` or `mysql.OpenStd(cfg)` / `postgres.OpenStd(cfg)` | Keep one DB pool for the process and create a fresh `session.Session` only when needed |
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

### Recommended application wiring

In a real service, do not open a new DB object on every query.
Treat `*gorm.DB` and `*sql.DB` as long-lived application dependencies, and create a fresh
`sqlcraft session.Session` only inside repository or service methods.

`session.Session` is stateful and not meant to be shared concurrently.

#### GORM-backed application

```go
import (
    "github.com/dennisge/sqlcraft/driver"
    mysqlhelper "github.com/dennisge/sqlcraft/driver/mysql"
    "github.com/dennisge/sqlcraft/session"
    "gorm.io/gorm"
)

type AppDB struct {
    Gorm *gorm.DB
}

func NewAppDB(dsn string) (*AppDB, error) {
    gdb, err := mysqlhelper.OpenGorm(driver.NormalizeConfig(&driver.Config{
        DSN: dsn,
    }))
    if err != nil {
        return nil, err
    }
    return &AppDB{Gorm: gdb}, nil
}

func (db *AppDB) SQL() session.Session {
    return mysqlhelper.NewGormSession(db.Gorm)
}
```

```go
type UserRepo struct {
    db *AppDB
}

func (r *UserRepo) List(status session.Optional[int]) ([]User, error) {
    var users []User
    err := r.db.SQL().
        Select("id", "user_name", "status").
        From("users").
        WhereSelective("status = #{status}", status).
        Scan(&users)
    return users, err
}

func (r *UserRepo) InTx(fn func(sess session.Session) error) error {
    return r.db.Gorm.Transaction(func(tx *gorm.DB) error {
        return fn(mysqlhelper.NewGormSession(tx))
    })
}
```

#### `database/sql`-backed application

```go
import (
    "database/sql"

    "github.com/dennisge/sqlcraft/driver"
    postgreshelper "github.com/dennisge/sqlcraft/driver/postgres"
    "github.com/dennisge/sqlcraft/session"
)

type AppDB struct {
    SQL *sql.DB
}

func NewAppDB(dsn string) (*AppDB, error) {
    sqldb, err := postgreshelper.OpenStd(driver.NormalizeConfig(&driver.Config{
        DSN: dsn,
    }))
    if err != nil {
        return nil, err
    }
    return &AppDB{SQL: sqldb}, nil
}

func (db *AppDB) SQLSession() session.Session {
    return postgreshelper.NewSession(db.SQL)
}
```

```go
type UserRepo struct {
    db *AppDB
}

func (r *UserRepo) List(enabled session.Optional[bool]) ([]User, error) {
    var users []User
    err := r.db.SQLSession().
        Select("id", "user_name", "enabled").
        From("users").
        WhereSelective("enabled = #{enabled}", enabled).
        Scan(&users)
    return users, err
}

func (r *UserRepo) InTx(fn func(sess session.Session) error) error {
    return r.db.SQLSession().Transaction(fn)
}
```

This pattern keeps connection pools stable, avoids rebuilding dialect state on every call,
and still gives each query a fresh fluent builder.

## Common Patterns

The following examples assume you already created `sess` using one of the snippets above.

### Query

```go
import "github.com/dennisge/sqlcraft/session"

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
    WhereSelective("user_name LIKE #{name}", session.Present(name)).
    OrderBy("id DESC").
    Limit(10).
    Scan(&users)
```

Use `session.Present(...)` when zero values such as `0`, `false`, `""`, or `nil`
still count as "the frontend explicitly sent this filter".

`session.Optional[T]` also supports direct JSON unmarshalling, which is convenient for API request DTOs:

```go
type ListUsersRequest struct {
    Status    session.Optional[int]        `json:"status"`
    Enabled   session.Optional[bool]       `json:"enabled"`
    DeletedAt session.Optional[*time.Time] `json:"deleted_at"`
}
```

- Missing field: `Present=false`
- JSON value like `0` or `false`: `Present=true`
- JSON `null`: `Present=true` and `IsNull()==true`
- For response DTOs, prefer `json:",omitzero"` when you want absent optionals omitted but explicit `null` preserved

```go
type UserResponse struct {
    DeletedAt session.Optional[*time.Time] `json:"deleted_at,omitzero"`
}
```

For equality-style Selective predicates, explicit JSON `null` is treated as "do not apply this predicate".
To query SQL `NULL`, write `IS NULL` or `IS NOT NULL` explicitly.

```go
err := sess.
    Select("id", "user_name").
    From("users").
    WhereSelective("status = #{status}", req.Status).
    WhereSelective("enabled = #{enabled}", req.Enabled).
    AppendRawSelective("AND deleted_at IS NULL", req.DeletedAt.IsNull()).
    Scan(&users)
```

### API DTO Example

The same `Optional[T]` type works well in HTTP request and response objects.

```go
import (
    "encoding/json"
    "net/http"
    "time"

    "github.com/dennisge/sqlcraft/session"
)

type ListUsersRequest struct {
    Status    session.Optional[int]        `json:"status"`
    Enabled   session.Optional[bool]       `json:"enabled"`
    DeletedAt session.Optional[*time.Time] `json:"deleted_at"`
}

type userRow struct {
    ID        int64
    UserName  string
    Status    int
    Enabled   bool
    DeletedAt *time.Time
}

type UserResponse struct {
    ID        int64                        `json:"id"`
    UserName  string                       `json:"user_name"`
    Status    int                          `json:"status"`
    Enabled   bool                         `json:"enabled"`
    DeletedAt session.Optional[*time.Time] `json:"deleted_at,omitzero"`
}

type Handler struct {
    sess session.Session
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
    var req ListUsersRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    var rows []userRow
    err := h.sess.Renew().
        Select("id", "user_name", "status", "enabled", "deleted_at").
        From("users").
        WhereSelective("status = #{status}", req.Status).
        WhereSelective("enabled = #{enabled}", req.Enabled).
        AppendRawSelective("AND deleted_at IS NULL", req.DeletedAt.IsNull()).
        Scan(&rows)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    resp := make([]UserResponse, 0, len(rows))
    for _, row := range rows {
        item := UserResponse{
            ID:       row.ID,
            UserName: row.UserName,
            Status:   row.Status,
            Enabled:  row.Enabled,
        }
        if row.DeletedAt != nil {
            item.DeletedAt = session.Present(row.DeletedAt)
        }
        resp = append(resp, item)
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(resp)
}
```

That handler gives you these API semantics:

- Request field omitted: matching predicate is skipped
- Request JSON like `{"status":0}`: keeps `status = 0`
- Request JSON like `{"enabled":false}`: keeps `enabled = false`
- Request JSON like `{"deleted_at":null}`: adds `AND deleted_at IS NULL`
- Response `DeletedAt` omitted when absent; use `session.Present[*time.Time](nil)` if you want to emit explicit JSON `null`

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

### Raw SQL Fragments

Use the raw helpers based on how much binding and pruning you want:

| Method | Binding style | Missing values | Good fit |
|--------|----------------|----------------|----------|
| `AppendRaw(...)` | Positional | Error if placeholder count does not match args | Small one-off fragments |
| `AppendRawNamed(...)` | Strict struct/map named binding | Error if a placeholder cannot be resolved | Reusable named filters where every placeholder is required |
| `AppendRawSelective(...)` | Positional or named | Prunes only the missing leaf predicates | Complex optional search filters |

```go
type AuditFilter struct {
    UserName session.Optional[string] `db:"user_name"`
    Status   session.Optional[int]    `db:"status"`
    From     session.Optional[string] `db:"from"`
    To       session.Optional[string] `db:"to"`
}

filter := AuditFilter{
    UserName: session.Present("alice"),
    Status:   session.Absent[int](),
    From:     session.Present("2024-01-01"),
    To:       session.Present("2024-02-01"),
}
lockForUpdate := true

err := sess.
    Select("id", "user_name").
    From("users").
    AppendRawSelective(
        "WHERE created_at BETWEEN #{from} AND #{to} AND user_name = #{user_name} AND status = #{status}",
        filter,
    ).
    AppendRawSelective("FOR UPDATE", lockForUpdate).
    Scan(&users)
```

That example becomes:

```sql
WHERE created_at BETWEEN #{from} AND #{to} AND user_name = #{user_name}
```

`status = #{status}` is pruned because `filter.Status` is absent, while the `BETWEEN #{from} AND #{to}` predicate stays intact as one clause.

If you need `0`, `false`, `""`, or `nil` to remain valid in a Selective call, wrap them with `session.Present(...)`.

```go
err := sess.
    Select("id").
    From("users").
    WhereSelective("status = #{status}", session.Present(0)).
    AppendRawSelective("AND enabled = #{enabled}", session.Present(false)).
    AppendRawSelective("AND deleted_at IS NULL", session.Present(true)).
    Scan(&users)
```

`AppendRawNamed(...)` is the strict variant: if a placeholder cannot be resolved from the provided struct or map,
it returns an error instead of silently skipping the fragment. It also does not rewrite SQL for you, so if a field is explicitly `null`,
write `IS NULL` in the fragment instead of `= #{field}`.

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
