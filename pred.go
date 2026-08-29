package sqlwhere

import (
	"fmt"
	"strings"
)

// Predicate is a parameterized boolean SQL fragment.
// Only constructors in this package implement it (sealed).
type Predicate interface {
	render(d Dialect, n int) (sql string, args []any, next int, err error)
}

type cmpPred struct {
	col string
	op  string
	val any
}

func (p cmpPred) render(d Dialect, n int) (string, []any, int, error) {
	if p.op == "ILIKE" && d != Postgres {
		return "", nil, n, ErrILikeDialect
	}
	ident, err := quoteIdent(p.col, d)
	if err != nil {
		return "", nil, n, err
	}
	ph, next := placeholder(d, n)
	return ident + " " + p.op + " " + ph, []any{p.val}, next, nil
}

// Eq is col = val.
func Eq(col string, val any) Predicate { return cmpPred{col: col, op: "=", val: val} }

// Neq is col <> val.
func Neq(col string, val any) Predicate { return cmpPred{col: col, op: "<>", val: val} }

// Gt is col > val.
func Gt(col string, val any) Predicate { return cmpPred{col: col, op: ">", val: val} }

// Gte is col >= val.
func Gte(col string, val any) Predicate { return cmpPred{col: col, op: ">=", val: val} }

// Lt is col < val.
func Lt(col string, val any) Predicate { return cmpPred{col: col, op: "<", val: val} }

// Lte is col <= val.
func Lte(col string, val any) Predicate { return cmpPred{col: col, op: "<=", val: val} }

// Like is col LIKE val. val belongs in args, including wildcards.
func Like(col string, val any) Predicate {
	return cmpPred{col: col, op: "LIKE", val: val}
}

// ILike is col ILIKE val. Build with Question returns ErrILikeDialect.
func ILike(col string, val any) Predicate {
	return cmpPred{col: col, op: "ILIKE", val: val}
}

type inPred struct {
	col  string
	vals []any
	not  bool
}

func inVals[T any](col string, vals []T) inPred {
	args := make([]any, len(vals))
	for i, v := range vals {
		args[i] = v
	}
	return inPred{col: col, vals: args}
}

// In is col IN (...). Empty vals yield ErrEmptyIn at Build. Works with ids... on typed slices.
func In[T any](col string, vals ...T) Predicate {
	return inVals(col, vals)
}

// NotIn is col NOT IN (...). Empty vals yield ErrEmptyIn at Build.
func NotIn[T any](col string, vals ...T) Predicate {
	p := inVals(col, vals)
	p.not = true
	return p
}

func (p inPred) render(d Dialect, n int) (string, []any, int, error) {
	if len(p.vals) == 0 {
		return "", nil, n, ErrEmptyIn
	}
	ident, err := quoteIdent(p.col, d)
	if err != nil {
		return "", nil, n, err
	}
	phs := make([]string, len(p.vals))
	args := make([]any, len(p.vals))
	for i, v := range p.vals {
		var ph string
		ph, n = placeholder(d, n)
		phs[i] = ph
		args[i] = v
	}
	op := " IN ("
	if p.not {
		op = " NOT IN ("
	}
	return ident + op + strings.Join(phs, ", ") + ")", args, n, nil
}

type betweenPred struct {
	col string
	lo  any
	hi  any
}

// Between is col BETWEEN lo AND hi.
func Between(col string, lo, hi any) Predicate {
	return betweenPred{col: col, lo: lo, hi: hi}
}

func (p betweenPred) render(d Dialect, n int) (string, []any, int, error) {
	ident, err := quoteIdent(p.col, d)
	if err != nil {
		return "", nil, n, err
	}
	lo, n := placeholder(d, n)
	hi, n := placeholder(d, n)
	return ident + " BETWEEN " + lo + " AND " + hi, []any{p.lo, p.hi}, n, nil
}

type nullPred struct {
	col    string
	notNil bool
}

// IsNull is col IS NULL.
func IsNull(col string) Predicate { return nullPred{col: col} }

// IsNotNull is col IS NOT NULL.
func IsNotNull(col string) Predicate { return nullPred{col: col, notNil: true} }

func (p nullPred) render(d Dialect, n int) (string, []any, int, error) {
	ident, err := quoteIdent(p.col, d)
	if err != nil {
		return "", nil, n, err
	}
	if p.notNil {
		return ident + " IS NOT NULL", nil, n, nil
	}
	return ident + " IS NULL", nil, n, nil
}

type boolPred struct {
	op string
	ps []Predicate
}

// And combines predicates with AND. Empty list yields ErrEmptyBool at Build.
func And(ps ...Predicate) Predicate { return boolPred{op: "AND", ps: ps} }

// Or combines predicates with OR. Empty list yields ErrEmptyBool at Build.
func Or(ps ...Predicate) Predicate { return boolPred{op: "OR", ps: ps} }

func (p boolPred) render(d Dialect, n int) (string, []any, int, error) {
	if len(p.ps) == 0 {
		return "", nil, n, ErrEmptyBool
	}
	parts := make([]string, 0, len(p.ps))
	var args []any
	for _, child := range p.ps {
		if child == nil {
			return "", nil, n, ErrNilPredicate
		}
		sql, childArgs, next, err := child.render(d, n)
		if err != nil {
			return "", nil, n, err
		}
		n = next
		parts = append(parts, sql)
		args = append(args, childArgs...)
	}
	if len(parts) == 1 {
		return parts[0], args, n, nil
	}
	return "(" + strings.Join(parts, " "+p.op+" ") + ")", args, n, nil
}

type notPred struct {
	p Predicate
}

// Not is NOT (p).
func Not(p Predicate) Predicate { return notPred{p: p} }

func (p notPred) render(d Dialect, n int) (string, []any, int, error) {
	if p.p == nil {
		return "", nil, n, ErrNilPredicate
	}
	sql, args, next, err := p.p.render(d, n)
	if err != nil {
		return "", nil, n, err
	}
	return "NOT (" + sql + ")", args, next, nil
}

type rawPred struct {
	sql  string
	args []any
}

// Raw is a trusted SQL fragment. Placeholders in the fragment must be ?.
// Do not put user input into the SQL text; pass it as args.
func Raw(sql string, args ...any) Predicate {
	return rawPred{sql: sql, args: args}
}

func (p rawPred) render(d Dialect, n int) (string, []any, int, error) {
	sql, next, count, err := rewriteRawPlaceholders(p.sql, d, n)
	if err != nil {
		return "", nil, n, err
	}
	if count != len(p.args) {
		return "", nil, n, fmt.Errorf("%w: got %d placeholders, %d args", ErrRawArgs, count, len(p.args))
	}
	out := make([]any, len(p.args))
	copy(out, p.args)
	return sql, out, next, nil
}

func joinAND(ps []Predicate, d Dialect, n int) (string, []any, int, error) {
	parts := make([]string, 0, len(ps))
	var args []any
	for _, p := range ps {
		if p == nil {
			return "", nil, n, ErrNilPredicate
		}
		sql, childArgs, next, err := p.render(d, n)
		if err != nil {
			return "", nil, n, err
		}
		n = next
		parts = append(parts, sql)
		args = append(args, childArgs...)
	}
	return strings.Join(parts, " AND "), args, n, nil
}
