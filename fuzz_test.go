package sqlwhere

import (
	"errors"
	"strings"
	"testing"
)

func FuzzEqValue(f *testing.F) {
	f.Add("hello")
	f.Add("'; DROP TABLE t; --")
	f.Add("$1")
	f.Add("?")
	f.Add("")
	f.Add(`"id"`)
	f.Add("a'b")
	f.Fuzz(func(t *testing.T, value string) {
		sql, args, err := On("SELECT id FROM items").And(Eq("name", value)).Build(Postgres)
		if err != nil {
			t.Fatal(err)
		}
		if len(args) != 1 || args[0] != value {
			t.Fatalf("args = %#v", args)
		}
		const expected = `SELECT id FROM items WHERE ("name" = $1)`
		if sql != expected {
			t.Fatalf("sql = %q", sql)
		}
		if value != "" && !strings.Contains(expected, value) && strings.Contains(sql, value) {
			t.Fatalf("value leaked into SQL: %q in %q", value, sql)
		}
	})
}

func FuzzIdent(f *testing.F) {
	f.Add("status")
	f.Add("u.created_at")
	f.Add("status;drop")
	f.Add("1id")
	f.Add("")
	f.Add("a.b.c")
	f.Fuzz(func(t *testing.T, ident string) {
		sql, args, err := On("SELECT 1 FROM t").And(Eq(ident, 1)).Build(Postgres)
		if err != nil {
			if !errors.Is(err, ErrInvalidIdent) {
				t.Fatalf("err = %v", err)
			}
			if sql != "" {
				t.Fatalf("sql = %q on error", sql)
			}
			return
		}
		if len(args) != 1 || args[0] != 1 {
			t.Fatalf("args = %#v", args)
		}
		quoted, qerr := quoteIdent(ident, Postgres)
		if qerr != nil {
			t.Fatalf("Build succeeded but quoteIdent failed: %v", qerr)
		}
		if !strings.Contains(sql, quoted+" = $1") {
			t.Fatalf("sql = %q, quoted = %q", sql, quoted)
		}
	})
}
