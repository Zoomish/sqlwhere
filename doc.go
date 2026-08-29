// Package sqlwhere builds parameterized WHERE, ORDER BY, LIMIT, and OFFSET
// clauses on top of SQL the caller already wrote.
//
// It is not an ORM and does not parse or rewrite SELECT lists. Filter values
// go only into args. Column names are allowlisted and quoted.
//
// Predicate is sealed: only constructors in this package (Eq, In, Raw, …)
// implement it. There is no supported way to satisfy Predicate from another
// package; trusted SQL fragments use Raw.
//
// Bind supplies arguments for placeholders already present in the base SQL.
// The required Bind length is max($n) for Postgres (so $1 and $3 without $2
// still need three arguments) and the count of ? for Question. Generated
// placeholders for predicates continue after that.
package sqlwhere
