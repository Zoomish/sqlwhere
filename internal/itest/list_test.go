package itest

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Zoomish/sqlwhere"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

func TestSQLiteList(t *testing.T) {
	db := openSQLite(t)
	seed(t, db, sqlwhere.Question)
	runListCases(t, db, sqlwhere.Question)
}

func TestPostgresList(t *testing.T) {
	db := openPostgres(t)
	if db == nil {
		return
	}
	_, _ = db.Exec(`DROP TABLE IF EXISTS products`)
	seed(t, db, sqlwhere.Postgres)
	runListCases(t, db, sqlwhere.Postgres)
}

func TestPostgresUpdateReturning(t *testing.T) {
	db := openPostgres(t)
	if db == nil {
		return
	}
	_, _ = db.Exec(`DROP TABLE IF EXISTS products`)
	seed(t, db, sqlwhere.Postgres)
	q, args, err := sqlwhere.On(`UPDATE products SET n = n WHERE tenant_id = $1 RETURNING id`).
		Bind("acme").
		And(sqlwhere.Eq("status", "active")).
		Build(sqlwhere.Postgres)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		t.Fatalf("query %s: %v", q, err)
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !equalIDsUnordered(ids, []int{1, 2}) {
		t.Fatalf("ids = %v", ids)
	}
}

func TestUnionDoesNotRun(t *testing.T) {
	_, _, err := sqlwhere.On("SELECT a FROM t UNION SELECT b FROM u").
		And(sqlwhere.Eq("a", 1)).
		Build(sqlwhere.Postgres)
	if !errors.Is(err, sqlwhere.ErrCompoundQuery) {
		t.Fatalf("err = %v, want ErrCompoundQuery", err)
	}
}

func runListCases(t *testing.T, db *sql.DB, d sqlwhere.Dialect) {
	t.Helper()
	ctx := context.Background()

	t.Run("tenant only", func(t *testing.T) {
		ids, err := ListProductIDs(ctx, db, d, ListFilter{TenantID: "acme", Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		if !equalIDs(ids, []int{1, 2, 3}) {
			t.Fatalf("ids = %v", ids)
		}
	})

	t.Run("status", func(t *testing.T) {
		ids, err := ListProductIDs(ctx, db, d, ListFilter{TenantID: "acme", Status: "active", Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		if !equalIDs(ids, []int{1, 2}) {
			t.Fatalf("ids = %v", ids)
		}
	})

	t.Run("nil minN", func(t *testing.T) {
		ids, err := ListProductIDs(ctx, db, d, ListFilter{TenantID: "acme", Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		if !equalIDs(ids, []int{1, 2, 3}) {
			t.Fatalf("ids = %v", ids)
		}
	})

	t.Run("empty ids", func(t *testing.T) {
		ids, err := ListProductIDs(ctx, db, d, ListFilter{TenantID: "acme", IDs: nil, Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		if !equalIDs(ids, []int{1, 2, 3}) {
			t.Fatalf("ids = %v", ids)
		}
	})

	t.Run("empty ids slice", func(t *testing.T) {
		ids, err := ListProductIDs(ctx, db, d, ListFilter{TenantID: "acme", IDs: []int{}, Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		if !equalIDs(ids, []int{1, 2, 3}) {
			t.Fatalf("ids = %v", ids)
		}
	})

	t.Run("combined", func(t *testing.T) {
		minN := 10
		ids, err := ListProductIDs(ctx, db, d, ListFilter{
			TenantID: "acme",
			Status:   "active",
			MinN:     &minN,
			IDs:      []int{1, 2, 3},
			Limit:    10,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !equalIDs(ids, []int{1, 2}) {
			t.Fatalf("ids = %v", ids)
		}
	})
}

func openSQLite(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	return db
}

func openPostgres(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("SQLWHERE_POSTGRES_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@127.0.0.1:5432/sqlwhere?sslmode=disable"
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		if os.Getenv("SQLWHERE_REQUIRE_POSTGRES") == "1" {
			t.Fatalf("postgres required: %v", err)
		}
		t.Skipf("postgres not available: %v", err)
	}
	return db
}

func seed(t *testing.T, db *sql.DB, d sqlwhere.Dialect) {
	t.Helper()
	_, err := db.Exec(`
CREATE TABLE products (
	id INTEGER NOT NULL,
	name TEXT NOT NULL,
	status TEXT,
	n INTEGER,
	tenant_id TEXT NOT NULL,
	deleted_at TEXT
)`)
	if err != nil {
		t.Fatal(err)
	}
	insert := `INSERT INTO products (id, name, status, n, tenant_id, deleted_at) VALUES `
	if d == sqlwhere.Question {
		insert += `(?, ?, ?, ?, ?, ?)`
	} else {
		insert += `($1, $2, $3, $4, $5, $6)`
	}
	rows := [][]any{
		{1, "alpha", "active", 10, "acme", nil},
		{2, "beta", "active", 20, "acme", nil},
		{3, "gamma", "hidden", 30, "acme", nil},
		{4, "other", "active", 40, "other", nil},
	}
	for _, r := range rows {
		if _, err := db.Exec(insert, r...); err != nil {
			t.Fatal(err)
		}
	}
}

func equalIDs(got, want []int) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func equalIDsUnordered(got, want []int) bool {
	if len(got) != len(want) {
		return false
	}
	seen := make(map[int]int, len(want))
	for _, id := range want {
		seen[id]++
	}
	for _, id := range got {
		seen[id]--
		if seen[id] < 0 {
			return false
		}
	}
	return true
}
