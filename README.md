# sqlwhere

Parameterized `WHERE` / `ORDER BY` / `LIMIT` on top of SQL you already wrote. Not an ORM.

[![CI](https://github.com/Zoomish/sqlwhere/actions/workflows/ci.yml/badge.svg)](https://github.com/Zoomish/sqlwhere/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/Zoomish/sqlwhere.svg)](https://pkg.go.dev/github.com/Zoomish/sqlwhere)
[![Go 1.22+](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go)](https://go.dev/dl/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

sqlc cannot express optional list filters. The usual workaround — `AND (NOT @has_status OR status = @status)` — fights indexes. Squirrel is in maintenance mode. Full query builders exist; most teams still want to keep their `SELECT`.

sqlwhere is the missing layer: predicates in, `(sql, args)` out. Values never enter the SQL text.

## Install

```bash
go get github.com/Zoomish/sqlwhere
```

## 30-second example

```go
q := sqlwhere.On(`SELECT id, name FROM products WHERE tenant_id = $1 AND deleted_at IS NULL`).
    Bind(tenantID).
    AndIf(status != "", sqlwhere.Eq("status", status)).
    AndIf(len(ids) > 0, sqlwhere.In("id", ids...)).
    Order(sqlwhere.Desc("created_at"), sqlwhere.Asc("id")).
    Limit(20)

query, args, err := q.Build(sqlwhere.Postgres)
if err != nil {
    return err
}
rows, err := pool.Query(ctx, query, args...)
```

Produces:

```sql
SELECT id, name FROM products WHERE tenant_id = $1 AND deleted_at IS NULL
  AND ("status" = $2 AND "id" IN ($3, $4))
  ORDER BY "created_at" DESC, "id" ASC
  LIMIT $5
```

`args` is `[tenantID, status, ids..., 20]`.

`Query` is not safe for concurrent use. `And` / `Bind` / `Order` clone. `AndIf(false)` and `AndIfFn(false, …)` return the same pointer without copying.

## Bind vs predicates

`Bind` supplies arguments for placeholders **already in the base SQL**. Predicates append new placeholders after those.

| Piece | Placeholders | Args |
| --- | --- | --- |
| `On("... WHERE tenant_id = $1").Bind(tenantID)` | `$1` in the string you wrote | first |
| `And(Eq("status", status))` | `$2` generated | after Bind |

`Build` errors if `len(Bind args)` does not match placeholders already in the base. It does not rewrite the base.

For Postgres the required length is **`max($n)`**, not the number of distinct placeholders. `$1` and `$3` without `$2` still need three Bind arguments (the unused slot is the caller's problem). For `Question`, the required length is the count of `?`.

Base SQL must use the same dialect you pass to `Build`. Mixing `$1` with `sqlwhere.Question` is an error.

## sqlc companion

sqlc generated functions do not export their SQL, so sqlwhere cannot wrap `q.ListProducts(ctx, arg)`. Keep sqlc for static queries (get-by-id, inserts). For list endpoints, keep the SQL next to the query — in the `.sql` file or a `const` — and scan into sqlc models:

```go
const listProducts = `
SELECT id, name, status, created_at
FROM products
WHERE tenant_id = $1 AND deleted_at IS NULL`

func (r *Repo) ListProducts(ctx context.Context, tenantID string, f Filter) ([]Product, error) {
    q := sqlwhere.On(listProducts).
        Bind(tenantID).
        AndIf(f.Status != "", sqlwhere.Eq("status", f.Status)).
        AndIf(len(f.IDs) > 0, sqlwhere.In("id", f.IDs...)).
        Order(sqlwhere.Desc("created_at")).
        Limit(f.Limit)

    query, args, err := q.Build(sqlwhere.Postgres)
    if err != nil {
        return nil, err
    }
    rows, err := r.pool.Query(ctx, query, args...)
    if err != nil {
        return nil, err
    }
    return pgx.CollectRows(rows, pgx.RowToStructByName[Product])
}
```

`sqlc.switch` (if it lands) expands a **fixed** set of compile-time branches. It does not replace runtime `AND` of optional filters.

## Safety

| Input | Where it goes |
| --- | --- |
| Filter values (`status`, IDs, search string) | `args` only |
| Column / table names | whitelist `[A-Za-z_][A-Za-z0-9_]*` per `.` segment, then dialect quoting |
| `Raw("similarity(name, ?) > 0.3", q)` | fragment is trusted SQL; `?` rewritten to `$n`; args still bound |

Empty `In()` is an error — skip it with `AndIf(len(ids) > 0, ...)`.

`AndIf`'s second argument is evaluated even when the condition is false (Go rules). Do not write `AndIf(p != nil, Eq("price", *p))`. Use `AndIfFn` or a plain `if`:

```go
q = q.AndIfFn(minPrice != nil, func() sqlwhere.Predicate {
    return sqlwhere.Gte("price", *minPrice)
})
```

## Dialects

One `Build` call:

- `sqlwhere.Postgres` — `$1`, `$2`, … and `"ident"`
- `sqlwhere.Question` — `?` and `` `ident` `` (MySQL / SQLite)

`ORDER BY` / `LIMIT` / `OFFSET` in the **outer** query conflict with `Order` / `Limit` / `Offset` on the builder (`ErrOrderConflict`, …). Inner subqueries are ignored. Extra predicates are inserted **before** an existing outer `ORDER BY` / `LIMIT` / `OFFSET` / `FOR UPDATE` / `GROUP BY` / `HAVING` / `WINDOW` / `RETURNING` / `FETCH`.

Attaching predicates, `Order`, `Limit`, or `Offset` to an outer `UNION` / `EXCEPT` / `INTERSECT`, to `INSERT`, or to `UPDATE ... FROM` returns `ErrCompoundQuery`. Filter those in SQL you control, or wrap the compound query as a subquery and attach to the outer `SELECT`.

## Comparison

| | sqlwhere | Squirrel | Relica / Quarry | sqlc `NOT @has_x OR` |
| --- | --- | --- | --- | --- |
| Optional `WHERE` | yes | yes (maintenance) | yes, plus the rest of SQL | yes, index-hostile |
| Keep your `SELECT` | yes | yes | no (full builder) | yes |
| Companion to sqlc | yes | glue | no | native, static |
| Zero production deps | yes | no | Relica: yes | n/a |
| Identifier allowlist | yes | caller | varies | n/a |
| HTTP query-string DSL | no | no | no | no |

sqlwhere wins as a **narrow layer**, not as “another SQL builder”.

## Not this

- JOIN / FROM / SELECT DSL
- struct scanning (use pgx `CollectRows` or scany)
- GORM
- HTTP filter language (`?name[eq]=`)
- Postgres RLS helpers

## API

See [pkg.go.dev/github.com/Zoomish/sqlwhere](https://pkg.go.dev/github.com/Zoomish/sqlwhere).

**Predicates:** `Eq` `Neq` `Gt` `Gte` `Lt` `Lte` `In` `NotIn` `Between` `Like` `ILike` `IsNull` `IsNotNull` `And` `Or` `Not` `Raw`

**Builder:** `On` `Bind` `And` `AndIf` `AndIfFn` `Order` (`Asc` / `Desc`) `Limit` `Offset` `Build`

`In` / `NotIn` are generic (`In("id", ids...)` works for `[]int`, `[]string`, …). `ILike` is Postgres-only.

## Compatibility

- Go 1.22 or later.
- Library source imports only the standard library. Integration drivers live in [`internal/itest`](internal/itest) (a nested module); `go get github.com/Zoomish/sqlwhere` does not download SQLite. Run those tests with `go test` inside `internal/itest`.
- After `v1.0.0`, minor releases (`v1.x`) are additive only. Breaking changes require a `v2` module path (`github.com/Zoomish/sqlwhere/v2`).
- `Predicate` is sealed. `Question` stays the `?` dialect name. Bind length stays `max($n)` / count of `?`.

See [CHANGELOG.md](CHANGELOG.md).

## Benchmark

`BenchmarkBuild` on an Intel i7-13620H (Windows/amd64), typical list query with Bind + Eq + In + Order + Limit:

```
BenchmarkBuild-16    478748    2340 ns/op    2320 B/op    53 allocs/op
```

This is string assembly, not a database round-trip. Numbers will move with Go and hardware; re-run `go test -bench=BenchmarkBuild -benchmem`.

## License

[MIT](LICENSE)
