# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `NotIn`, `Between`, `ILike` (`ILike` requires `Postgres`; `Question` returns `ErrILikeDialect`).
- `ErrCompoundQuery` when attaching clauses to outer `UNION` / `EXCEPT` / `INTERSECT`, `INSERT`, or `UPDATE ... FROM`.
- Package godoc: sealed `Predicate`, Bind length is `max($n)` for Postgres.
- Nested module `internal/itest` for SQLite/Postgres integration (not a dependency of the published module).
- CI: `gofmt`, short fuzz, integration job with Postgres.

### Changed

- Predicates insert before outer `GROUP BY` / `HAVING` / `WINDOW` / `RETURNING` / `FETCH`.
- `UPDATE t SET from = …` is not treated as `UPDATE ... FROM`.
- `On` strips all trailing semicolons.
- README Compatibility: Go 1.22+; v1.x additive; breaking changes go to `/v2`.
- Integration tests moved out of the root module; `go get` no longer downloads SQLite.
