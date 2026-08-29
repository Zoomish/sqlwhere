package sqlwhere

import (
	"strconv"
	"strings"
)

// Dialect selects placeholder style and identifier quoting.
type Dialect int

const (
	// Postgres uses $1, $2, … and quoted "ident".
	Postgres Dialect = iota
	// Question uses ? and quoted `ident` (MySQL and SQLite).
	Question
)

func (d Dialect) valid() bool {
	return d == Postgres || d == Question
}

func placeholder(d Dialect, n int) (string, int) {
	if d == Question {
		return "?", n + 1
	}
	return "$" + strconv.Itoa(n), n + 1
}

func quotePart(part string, d Dialect) string {
	if d == Question {
		return "`" + strings.ReplaceAll(part, "`", "``") + "`"
	}
	return `"` + strings.ReplaceAll(part, `"`, `""`) + `"`
}
