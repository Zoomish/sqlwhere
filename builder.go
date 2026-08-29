package sqlwhere

import (
	"fmt"
	"strings"
)

// Order is a single ORDER BY term. Use Asc or Desc; fields are opaque.
type Order struct {
	col  string
	desc bool
}

// Asc orders col ascending.
func Asc(col string) Order { return Order{col: col} }

// Desc orders col descending.
func Desc(col string) Order { return Order{col: col, desc: true} }

// Query is a SQL fragment builder. It is not safe for concurrent use.
// Mutating methods clone; the original Query is left unchanged.
type Query struct {
	base   string
	bind   []any
	preds  []Predicate
	orders []Order
	limit  *int
	offset *int
}

// On starts a builder from base SQL. Trailing semicolons and space are stripped.
func On(base string) *Query {
	base = strings.TrimSpace(base)
	for strings.HasSuffix(base, ";") {
		base = strings.TrimSpace(strings.TrimSuffix(base, ";"))
	}
	return &Query{base: base}
}

func (q *Query) clone() *Query {
	n := *q
	if q.bind != nil {
		n.bind = append([]any(nil), q.bind...)
	}
	if q.preds != nil {
		n.preds = append([]Predicate(nil), q.preds...)
	}
	if q.orders != nil {
		n.orders = append([]Order(nil), q.orders...)
	}
	if q.limit != nil {
		v := *q.limit
		n.limit = &v
	}
	if q.offset != nil {
		v := *q.offset
		n.offset = &v
	}
	return &n
}

// Bind sets arguments for placeholders already in the base SQL.
// Postgres: len(args) must equal max($n) in the base (gaps count).
// Question: len(args) must equal the number of ? in the base.
func (q *Query) Bind(args ...any) *Query {
	n := q.clone()
	n.bind = append([]any(nil), args...)
	return n
}

// And appends a predicate with AND.
func (q *Query) And(p Predicate) *Query {
	n := q.clone()
	n.preds = append(n.preds, p)
	return n
}

// AndIf appends p when ok is true. The predicate argument is always evaluated;
// do not dereference a nil pointer there — use AndIfFn.
func (q *Query) AndIf(ok bool, p Predicate) *Query {
	if !ok {
		return q
	}
	return q.And(p)
}

// AndIfFn calls p and appends the result only when ok is true.
func (q *Query) AndIfFn(ok bool, p func() Predicate) *Query {
	if !ok || p == nil {
		return q
	}
	return q.And(p())
}

// Order appends ORDER BY terms. Errors if the outer query already has ORDER BY.
func (q *Query) Order(cols ...Order) *Query {
	n := q.clone()
	n.orders = append(n.orders, cols...)
	return n
}

// Limit sets LIMIT. n must be >= 0. Errors if the outer query already has LIMIT.
func (q *Query) Limit(n int) *Query {
	q2 := q.clone()
	q2.limit = &n
	return q2
}

// Offset sets OFFSET. n must be >= 0. Errors if the outer query already has OFFSET.
func (q *Query) Offset(n int) *Query {
	q2 := q.clone()
	q2.offset = &n
	return q2
}

// Build renders SQL and args for d. The base SQL is not rewritten.
func (q *Query) Build(d Dialect) (string, []any, error) {
	if q == nil || q.base == "" {
		return "", nil, ErrEmptySQL
	}
	if !d.valid() {
		return "", nil, ErrUnknownDialect
	}
	info := analyzeSQL(q.base)
	if info.maxDollar > 0 && info.questionN > 0 {
		return "", nil, ErrMixedPlaceholders
	}
	if d == Postgres && info.questionN > 0 {
		return "", nil, ErrDialectPlaceholder
	}
	if d == Question && info.maxDollar > 0 {
		return "", nil, ErrDialectPlaceholder
	}
	want := info.maxDollar
	if d == Question {
		want = info.questionN
	}
	if len(q.bind) != want {
		return "", nil, fmt.Errorf("%w: got %d, want %d", ErrBindCount, len(q.bind), want)
	}
	if len(q.orders) > 0 && info.hasOuterOrder {
		return "", nil, ErrOrderConflict
	}
	if q.limit != nil && info.hasOuterLimit {
		return "", nil, ErrLimitConflict
	}
	if q.offset != nil && info.hasOuterOffset {
		return "", nil, ErrOffsetConflict
	}
	if q.limit != nil && *q.limit < 0 {
		return "", nil, ErrNegativeLimit
	}
	if q.offset != nil && *q.offset < 0 {
		return "", nil, ErrNegativeOffset
	}
	attaches := len(q.preds) > 0 || len(q.orders) > 0 || q.limit != nil || q.offset != nil
	if attaches && (info.compound || info.insert || info.updateFrom) {
		return "", nil, ErrCompoundQuery
	}

	n := 1
	if d == Postgres {
		n = info.maxDollar + 1
	}
	var predSQL string
	var predArgs []any
	if len(q.preds) > 0 {
		sql, args, next, err := joinAND(q.preds, d, n)
		if err != nil {
			return "", nil, err
		}
		n = next
		predSQL = sql
		predArgs = args
	}

	var orderSQL string
	if len(q.orders) > 0 {
		parts := make([]string, 0, len(q.orders))
		for _, o := range q.orders {
			ident, err := quoteIdent(o.col, d)
			if err != nil {
				return "", nil, err
			}
			if o.desc {
				parts = append(parts, ident+" DESC")
			} else {
				parts = append(parts, ident+" ASC")
			}
		}
		orderSQL = strings.Join(parts, ", ")
	}

	args := append([]any(nil), q.bind...)
	args = append(args, predArgs...)

	var limitSQL, offsetSQL string
	if q.limit != nil {
		ph, next := placeholder(d, n)
		n = next
		limitSQL = ph
		args = append(args, *q.limit)
	}
	if q.offset != nil {
		ph, _ := placeholder(d, n)
		offsetSQL = ph
		args = append(args, *q.offset)
	}

	left := strings.TrimRight(q.base[:info.insertAt], " \t\n\r")
	right := strings.TrimLeft(q.base[info.insertAt:], " \t\n\r")

	var b strings.Builder
	b.Grow(len(q.base) + len(predSQL) + len(orderSQL) + 64)
	b.WriteString(left)
	if predSQL != "" {
		if info.hasOuterWhere {
			b.WriteString(" AND (")
			b.WriteString(predSQL)
			b.WriteString(")")
		} else {
			b.WriteString(" WHERE (")
			b.WriteString(predSQL)
			b.WriteString(")")
		}
	}
	if orderSQL != "" {
		b.WriteString(" ORDER BY ")
		b.WriteString(orderSQL)
	}
	if limitSQL != "" {
		b.WriteString(" LIMIT ")
		b.WriteString(limitSQL)
	}
	if offsetSQL != "" {
		b.WriteString(" OFFSET ")
		b.WriteString(offsetSQL)
	}
	if right != "" {
		b.WriteByte(' ')
		b.WriteString(right)
	}
	return b.String(), args, nil
}
