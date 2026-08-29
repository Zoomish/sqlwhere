package sqlwhere

import (
	"errors"
	"testing"
)

func TestQuoteIdent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		ident   string
		d       Dialect
		want    string
		wantErr error
	}{
		{name: "simple postgres", ident: "status", d: Postgres, want: `"status"`},
		{name: "qualified postgres", ident: "u.created_at", d: Postgres, want: `"u"."created_at"`},
		{name: "schema table col", ident: "public.u.id", d: Postgres, want: `"public"."u"."id"`},
		{name: "mysql", ident: "status", d: Question, want: "`status`"},
		{name: "mysql qualified", ident: "u.created_at", d: Question, want: "`u`.`created_at`"},
		{name: "underscore", ident: "_id", d: Postgres, want: `"_id"`},
		{name: "injection semicolon", ident: "status; drop table", d: Postgres, wantErr: ErrInvalidIdent},
		{name: "injection comment", ident: "u.x;--", d: Postgres, wantErr: ErrInvalidIdent},
		{name: "quoted ident", ident: `"id"`, d: Postgres, wantErr: ErrInvalidIdent},
		{name: "space", ident: "created at", d: Postgres, wantErr: ErrInvalidIdent},
		{name: "empty", ident: "", d: Postgres, wantErr: ErrInvalidIdent},
		{name: "dot empty", ident: "u.", d: Postgres, wantErr: ErrInvalidIdent},
		{name: "leading digit", ident: "1id", d: Postgres, wantErr: ErrInvalidIdent},
		{name: "dash", ident: "created-at", d: Postgres, wantErr: ErrInvalidIdent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := quoteIdent(tt.ident, tt.d)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				if got != "" {
					t.Fatalf("got %q, want empty on error", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildRejectsInjectedIdent(t *testing.T) {
	t.Parallel()
	injections := []string{
		"status; drop table t",
		`"id"`,
		"u.x;--",
		"id FROM users; --",
		"a b",
	}
	for _, ident := range injections {
		_, _, err := On("SELECT 1 FROM t").And(Eq(ident, 1)).Build(Postgres)
		if !errors.Is(err, ErrInvalidIdent) {
			t.Fatalf("ident %q: err = %v, want ErrInvalidIdent", ident, err)
		}
	}
}
