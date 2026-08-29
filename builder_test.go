package sqlwhere

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestBuildTable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		build   func() (string, []any, error)
		wantSQL string
		wantArg []any
		wantErr error
	}{
		{
			name: "no filters",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t").Build(Postgres)
			},
			wantSQL: "SELECT id FROM t",
			wantArg: []any{},
		},
		{
			name: "bind only",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t WHERE tenant_id = $1").Bind("acme").Build(Postgres)
			},
			wantSQL: "SELECT id FROM t WHERE tenant_id = $1",
			wantArg: []any{"acme"},
		},
		{
			name: "bind and eq",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t WHERE tenant_id = $1").
					Bind("acme").
					And(Eq("status", "active")).
					Build(Postgres)
			},
			wantSQL: `SELECT id FROM t WHERE tenant_id = $1 AND ("status" = $2)`,
			wantArg: []any{"acme", "active"},
		},
		{
			name: "no outer where",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t").And(Eq("status", "active")).Build(Postgres)
			},
			wantSQL: `SELECT id FROM t WHERE ("status" = $1)`,
			wantArg: []any{"active"},
		},
		{
			name: "subquery inner where",
			build: func() (string, []any, error) {
				return On("SELECT id FROM (SELECT id FROM u WHERE active = true) AS x").
					And(Eq("status", "x")).
					Build(Postgres)
			},
			wantSQL: `SELECT id FROM (SELECT id FROM u WHERE active = true) AS x WHERE ("status" = $1)`,
			wantArg: []any{"x"},
		},
		{
			name: "insert before order by",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t ORDER BY id").
					And(Eq("status", "x")).
					Build(Postgres)
			},
			wantSQL: `SELECT id FROM t WHERE ("status" = $1) ORDER BY id`,
			wantArg: []any{"x"},
		},
		{
			name: "andif false skipped",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t").
					AndIf(false, Eq("status", "x")).
					AndIf(true, Eq("kind", "a")).
					Build(Postgres)
			},
			wantSQL: `SELECT id FROM t WHERE ("kind" = $1)`,
			wantArg: []any{"a"},
		},
		{
			name: "and or nest",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t").
					And(Or(Eq("a", 1), Eq("b", 2))).
					And(Gt("n", 3)).
					Build(Postgres)
			},
			wantSQL: `SELECT id FROM t WHERE (("a" = $1 OR "b" = $2) AND "n" > $3)`,
			wantArg: []any{1, 2, 3},
		},
		{
			name: "like isnull in",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t").
					And(Like("name", "%x%")).
					And(IsNull("deleted_at")).
					And(In("id", 1, 2)).
					Build(Postgres)
			},
			wantSQL: `SELECT id FROM t WHERE ("name" LIKE $1 AND "deleted_at" IS NULL AND "id" IN ($2, $3))`,
			wantArg: []any{"%x%", 1, 2},
		},
		{
			name: "not neq gte lte is not null",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t").
					And(Not(Neq("a", 1))).
					And(Gte("p", 2)).
					And(Lte("p", 9)).
					And(Lt("q", 0)).
					And(IsNotNull("z")).
					Build(Postgres)
			},
			wantSQL: `SELECT id FROM t WHERE (NOT ("a" <> $1) AND "p" >= $2 AND "p" <= $3 AND "q" < $4 AND "z" IS NOT NULL)`,
			wantArg: []any{1, 2, 9, 0},
		},
		{
			name: "placeholder numbering continues",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t WHERE a = $1 AND b = $2").
					Bind(10, 20).
					And(Eq("c", 30)).
					And(Eq("d", 40)).
					Build(Postgres)
			},
			wantSQL: `SELECT id FROM t WHERE a = $1 AND b = $2 AND ("c" = $3 AND "d" = $4)`,
			wantArg: []any{10, 20, 30, 40},
		},
		{
			name: "question dialect",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t WHERE a = ?").
					Bind("x").
					And(Eq("b", "y")).
					Limit(5).
					Build(Question)
			},
			wantSQL: "SELECT id FROM t WHERE a = ? AND (`b` = ?) LIMIT ?",
			wantArg: []any{"x", "y", 5},
		},
		{
			name: "order limit offset",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t").
					And(Eq("status", "a")).
					Order(Desc("created_at"), Asc("id")).
					Limit(20).
					Offset(5).
					Build(Postgres)
			},
			wantSQL: `SELECT id FROM t WHERE ("status" = $1) ORDER BY "created_at" DESC, "id" ASC LIMIT $2 OFFSET $3`,
			wantArg: []any{"a", 20, 5},
		},
		{
			name: "raw",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t").
					And(Raw("similarity(name, ?) > 0.3", "foo")).
					Build(Postgres)
			},
			wantSQL: `SELECT id FROM t WHERE (similarity(name, $1) > 0.3)`,
			wantArg: []any{"foo"},
		},
		{
			name: "notin between ilike",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t").
					And(NotIn("id", 1, 2)).
					And(Between("n", 3, 9)).
					And(ILike("name", "%x%")).
					Build(Postgres)
			},
			wantSQL: `SELECT id FROM t WHERE ("id" NOT IN ($1, $2) AND "n" BETWEEN $3 AND $4 AND "name" ILIKE $5)`,
			wantArg: []any{1, 2, 3, 9, "%x%"},
		},
		{
			name: "ilike question",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t").And(ILike("name", "%x%")).Build(Question)
			},
			wantErr: ErrILikeDialect,
		},
		{
			name: "union attach",
			build: func() (string, []any, error) {
				return On("SELECT a FROM t UNION SELECT b FROM u").And(Eq("a", 1)).Build(Postgres)
			},
			wantErr: ErrCompoundQuery,
		},
		{
			name: "union bind only",
			build: func() (string, []any, error) {
				return On("SELECT a FROM t WHERE x = $1 UNION SELECT b FROM u").Bind(1).Build(Postgres)
			},
			wantSQL: "SELECT a FROM t WHERE x = $1 UNION SELECT b FROM u",
			wantArg: []any{1},
		},
		{
			name: "union in subquery attach ok",
			build: func() (string, []any, error) {
				return On("SELECT id FROM (SELECT a FROM t UNION SELECT b FROM u) AS x").
					And(Eq("id", 1)).
					Build(Postgres)
			},
			wantSQL: `SELECT id FROM (SELECT a FROM t UNION SELECT b FROM u) AS x WHERE ("id" = $1)`,
			wantArg: []any{1},
		},
		{
			name: "insert attach",
			build: func() (string, []any, error) {
				return On("INSERT INTO t (a) VALUES ($1)").Bind("x").And(Eq("a", "y")).Build(Postgres)
			},
			wantErr: ErrCompoundQuery,
		},
		{
			name: "insert bind only",
			build: func() (string, []any, error) {
				return On("INSERT INTO t (a) VALUES ($1)").Bind("x").Build(Postgres)
			},
			wantSQL: "INSERT INTO t (a) VALUES ($1)",
			wantArg: []any{"x"},
		},
		{
			name: "update from attach",
			build: func() (string, []any, error) {
				return On("UPDATE t SET a = 1 FROM u WHERE t.id = u.id").And(Eq("a", 2)).Build(Postgres)
			},
			wantErr: ErrCompoundQuery,
		},
		{
			name: "update attach ok",
			build: func() (string, []any, error) {
				return On("UPDATE t SET a = 1 WHERE tenant_id = $1").
					Bind("acme").
					And(Eq("status", "x")).
					Build(Postgres)
			},
			wantSQL: `UPDATE t SET a = 1 WHERE tenant_id = $1 AND ("status" = $2)`,
			wantArg: []any{"acme", "x"},
		},
		{
			name: "bind max dollar gap",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t WHERE a = $1 AND b = $3").Bind(1, 2, 3).Build(Postgres)
			},
			wantSQL: "SELECT id FROM t WHERE a = $1 AND b = $3",
			wantArg: []any{1, 2, 3},
		},
		{
			name: "group by insert before",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t GROUP BY id").And(Eq("status", "x")).Build(Postgres)
			},
			wantSQL: `SELECT id FROM t WHERE ("status" = $1) GROUP BY id`,
			wantArg: []any{"x"},
		},
		{
			name: "having insert before group",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t GROUP BY id HAVING count(*) > 1").And(Eq("status", "x")).Build(Postgres)
			},
			wantSQL: `SELECT id FROM t WHERE ("status" = $1) GROUP BY id HAVING count(*) > 1`,
			wantArg: []any{"x"},
		},
		{
			name: "fetch insert before",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t FETCH FIRST 10 ROWS ONLY").And(Eq("status", "x")).Build(Postgres)
			},
			wantSQL: `SELECT id FROM t WHERE ("status" = $1) FETCH FIRST 10 ROWS ONLY`,
			wantArg: []any{"x"},
		},
		{
			name: "window insert before",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t WINDOW w AS (ORDER BY id)").And(Eq("status", "x")).Build(Postgres)
			},
			wantSQL: `SELECT id FROM t WHERE ("status" = $1) WINDOW w AS (ORDER BY id)`,
			wantArg: []any{"x"},
		},
		{
			name: "returning insert before",
			build: func() (string, []any, error) {
				return On("UPDATE t SET a = 1 WHERE tenant_id = $1 RETURNING id").
					Bind("acme").
					And(Eq("status", "x")).
					Build(Postgres)
			},
			wantSQL: `UPDATE t SET a = 1 WHERE tenant_id = $1 AND ("status" = $2) RETURNING id`,
			wantArg: []any{"acme", "x"},
		},
		{
			name: "set from column not update from",
			build: func() (string, []any, error) {
				return On("UPDATE t SET from = 1 WHERE id = $1").
					Bind(1).
					And(Eq("status", "x")).
					Build(Postgres)
			},
			wantSQL: `UPDATE t SET from = 1 WHERE id = $1 AND ("status" = $2)`,
			wantArg: []any{1, "x"},
		},
		{
			name: "trailing semicolons",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t ;; ").And(Eq("a", 1)).Build(Postgres)
			},
			wantSQL: `SELECT id FROM t WHERE ("a" = $1)`,
			wantArg: []any{1},
		},
		{
			name: "empty in",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t").And(In[int]("id")).Build(Postgres)
			},
			wantErr: ErrEmptyIn,
		},
		{
			name: "order conflict",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t ORDER BY id").Order(Asc("name")).Build(Postgres)
			},
			wantErr: ErrOrderConflict,
		},
		{
			name: "limit conflict",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t LIMIT 1").Limit(10).Build(Postgres)
			},
			wantErr: ErrLimitConflict,
		},
		{
			name: "offset conflict",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t OFFSET 1").Offset(2).Build(Postgres)
			},
			wantErr: ErrOffsetConflict,
		},
		{
			name: "bind count",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t WHERE a = $1 AND b = $2").Bind(1).Build(Postgres)
			},
			wantErr: ErrBindCount,
		},
		{
			name: "mixed placeholders",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t WHERE a = $1 AND b = ?").Bind(1, 2).Build(Postgres)
			},
			wantErr: ErrMixedPlaceholders,
		},
		{
			name: "dialect mismatch",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t WHERE a = $1").Bind(1).Build(Question)
			},
			wantErr: ErrDialectPlaceholder,
		},
		{
			name: "negative limit",
			build: func() (string, []any, error) {
				return On("SELECT id FROM t").Limit(-1).Build(Postgres)
			},
			wantErr: ErrNegativeLimit,
		},
		{
			name: "immutable andif",
			build: func() (string, []any, error) {
				base := On("SELECT id FROM t")
				_ = base.And(Eq("a", 1))
				return base.And(Eq("b", 2)).Build(Postgres)
			},
			wantSQL: `SELECT id FROM t WHERE ("b" = $1)`,
			wantArg: []any{2},
		},
		{
			name: "andif fn nil pointer",
			build: func() (string, []any, error) {
				var minPrice *int
				return On("SELECT id FROM t").
					AndIfFn(minPrice != nil, func() Predicate { return Gte("price", *minPrice) }).
					And(Eq("kind", "a")).
					Build(Postgres)
			},
			wantSQL: `SELECT id FROM t WHERE ("kind" = $1)`,
			wantArg: []any{"a"},
		},
		{
			name: "postgres json path bind and order",
			build: func() (string, []any, error) {
				return On(`SELECT id FROM t WHERE payload #> $1 ORDER BY id`).
					Bind(`{"a"}`).
					And(Eq("status", "x")).
					Build(Postgres)
			},
			wantSQL: `SELECT id FROM t WHERE payload #> $1 AND ("status" = $2) ORDER BY id`,
			wantArg: []any{`{"a"}`, "x"},
		},
		{
			name: "postgres json path literal and order",
			build: func() (string, []any, error) {
				return On(`SELECT id FROM t WHERE payload #> '{"a"}' ORDER BY id`).
					And(Eq("status", "x")).
					Build(Postgres)
			},
			wantSQL: `SELECT id FROM t WHERE payload #> '{"a"}' AND ("status" = $1) ORDER BY id`,
			wantArg: []any{"x"},
		},
		{
			name: "plan example",
			build: func() (string, []any, error) {
				tenantID := "acme"
				status := "active"
				var minPrice *int
				ids := []int{1, 2}
				return On(`SELECT id, name FROM products WHERE tenant_id = $1 AND deleted_at IS NULL`).
					Bind(tenantID).
					AndIf(status != "", Eq("status", status)).
					AndIfFn(minPrice != nil, func() Predicate { return Gte("price", *minPrice) }).
					AndIf(len(ids) > 0, In("id", ids...)).
					Order(Desc("created_at"), Asc("id")).
					Limit(20).
					Build(Postgres)
			},
			wantSQL: `SELECT id, name FROM products WHERE tenant_id = $1 AND deleted_at IS NULL AND ("status" = $2 AND "id" IN ($3, $4)) ORDER BY "created_at" DESC, "id" ASC LIMIT $5`,
			wantArg: []any{"acme", "active", 1, 2, 20},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sql, args, err := tt.build()
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				if sql != "" {
					t.Fatalf("sql = %q on error", sql)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if sql != tt.wantSQL {
				t.Fatalf("sql\n got %q\nwant %q", sql, tt.wantSQL)
			}
			if args == nil {
				args = []any{}
			}
			if !reflect.DeepEqual(args, tt.wantArg) {
				t.Fatalf("args got %#v want %#v", args, tt.wantArg)
			}
		})
	}
}

func TestRawArgMismatch(t *testing.T) {
	t.Parallel()
	_, _, err := On("SELECT 1 FROM t").And(Raw("x = ?", 1, 2)).Build(Postgres)
	if !errors.Is(err, ErrRawArgs) {
		t.Fatalf("err = %v, want ErrRawArgs", err)
	}
}

func TestNilPredicate(t *testing.T) {
	t.Parallel()
	_, _, err := On("SELECT 1 FROM t").And(nil).Build(Postgres)
	if !errors.Is(err, ErrNilPredicate) {
		t.Fatalf("err = %v, want ErrNilPredicate", err)
	}
}

func TestEmptySQL(t *testing.T) {
	t.Parallel()
	_, _, err := On("  ; ").Build(Postgres)
	if !errors.Is(err, ErrEmptySQL) {
		t.Fatalf("err = %v, want ErrEmptySQL", err)
	}
}

func TestUnknownDialect(t *testing.T) {
	t.Parallel()
	_, _, err := On("SELECT 1 FROM t").Build(Dialect(9))
	if !errors.Is(err, ErrUnknownDialect) {
		t.Fatalf("err = %v, want ErrUnknownDialect", err)
	}
}

func TestValueNotInSQL(t *testing.T) {
	t.Parallel()
	payload := "'; DROP TABLE t; --"
	sql, args, err := On("SELECT id FROM items").And(Eq("name", payload)).Build(Postgres)
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 1 || args[0] != payload {
		t.Fatalf("args = %#v", args)
	}
	if strings.Contains(sql, payload) {
		t.Fatalf("payload leaked into SQL: %q", sql)
	}
}
