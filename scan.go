package sqlwhere

import (
	"strings"
)

type analyzed struct {
	hasOuterWhere  bool
	hasOuterOrder  bool
	hasOuterLimit  bool
	hasOuterOffset bool
	compound       bool
	insert         bool
	updateFrom     bool
	insertAt       int
	maxDollar      int
	questionN      int
}

func analyzeSQL(s string) analyzed {
	a := analyzed{insertAt: len(s)}
	depth := 0
	seenRel := false
	updateStmt := false
	orderPos, limitPos, offsetPos, forPos := -1, -1, -1, -1
	groupPos, havingPos, windowPos, returningPos, fetchPos := -1, -1, -1, -1, -1
	i := 0
	for i < len(s) {
		if s[i] == '\'' {
			i = skipQuoted(s, i, '\'')
			continue
		}
		if s[i] == '"' {
			i = skipQuoted(s, i, '"')
			continue
		}
		if s[i] == '`' {
			i = skipQuoted(s, i, '`')
			continue
		}
		if i+1 < len(s) && s[i] == '-' && s[i+1] == '-' {
			i = skipLineComment(s, i)
			continue
		}
		if s[i] == '#' {
			i = skipLineComment(s, i)
			continue
		}
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '*' {
			i = skipBlockComment(s, i)
			continue
		}
		if s[i] == '$' {
			if i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9' {
				n, next := parseDollar(s, i)
				if n > a.maxDollar {
					a.maxDollar = n
				}
				i = next
				continue
			}
			tag, next, ok := readDollarTag(s, i)
			if ok {
				i = skipDollarQuote(s, next, tag)
				continue
			}
			i++
			continue
		}
		if s[i] == '?' {
			a.questionN++
			i++
			continue
		}
		if s[i] == '(' {
			depth++
			i++
			continue
		}
		if s[i] == ')' {
			if depth > 0 {
				depth--
			}
			i++
			continue
		}
		if identStart(s[i]) && (i == 0 || !identPart(s[i-1])) {
			kw, end := readIdent(s, i)
			if depth == 0 {
				switch upperASCII(kw) {
				case "INSERT":
					a.insert = true
					seenRel = true
				case "UPDATE":
					seenRel = true
					a.updateFrom = false
					updateStmt = true
				case "FROM":
					seenRel = true
					if updateStmt {
						j := skipSpaceAndComments(s, end)
						if j >= len(s) || s[j] != '=' {
							a.updateFrom = true
						}
					}
				case "DELETE":
					seenRel = true
				case "UNION", "EXCEPT", "INTERSECT":
					a.compound = true
				case "WHERE":
					if seenRel {
						a.hasOuterWhere = true
					}
				case "ORDER":
					j := skipSpaceAndComments(s, end)
					by, byEnd := readIdent(s, j)
					if seenRel && orderPos < 0 && upperASCII(by) == "BY" {
						orderPos = i
						end = byEnd
					}
				case "LIMIT":
					if seenRel && limitPos < 0 {
						limitPos = i
					}
				case "OFFSET":
					if seenRel && offsetPos < 0 {
						offsetPos = i
					}
				case "FOR":
					j := skipSpaceAndComments(s, end)
					next, _ := readIdent(s, j)
					n := upperASCII(next)
					if seenRel && forPos < 0 && (n == "UPDATE" || n == "SHARE" || n == "NO" || n == "KEY") {
						forPos = i
					}
				case "GROUP":
					j := skipSpaceAndComments(s, end)
					by, byEnd := readIdent(s, j)
					if seenRel && groupPos < 0 && upperASCII(by) == "BY" {
						groupPos = i
						end = byEnd
					}
				case "HAVING":
					if seenRel && havingPos < 0 {
						havingPos = i
					}
				case "WINDOW":
					if seenRel && windowPos < 0 {
						windowPos = i
					}
				case "RETURNING":
					if seenRel && returningPos < 0 {
						returningPos = i
					}
				case "FETCH":
					if seenRel && fetchPos < 0 {
						fetchPos = i
					}
				}
			}
			i = end
			continue
		}
		i++
	}
	insertAt := len(s)
	for _, p := range []int{orderPos, limitPos, offsetPos, forPos, groupPos, havingPos, windowPos, returningPos, fetchPos} {
		if p >= 0 && p < insertAt {
			insertAt = p
		}
	}
	a.insertAt = insertAt
	a.hasOuterOrder = orderPos >= 0
	a.hasOuterLimit = limitPos >= 0
	a.hasOuterOffset = offsetPos >= 0
	return a
}

func identStart(c byte) bool {
	return c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func identPart(c byte) bool {
	return identStart(c) || (c >= '0' && c <= '9')
}

func readIdent(s string, i int) (string, int) {
	if i >= len(s) || !identStart(s[i]) {
		return "", i
	}
	j := i + 1
	for j < len(s) && identPart(s[j]) {
		j++
	}
	return s[i:j], j
}

func upperASCII(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func skipQuoted(s string, i int, q byte) int {
	i++
	for i < len(s) {
		if s[i] == q {
			if i+1 < len(s) && s[i+1] == q {
				i += 2
				continue
			}
			return i + 1
		}
		i++
	}
	return len(s)
}

func skipLineComment(s string, i int) int {
	for i < len(s) && s[i] != '\n' {
		i++
	}
	return i
}

func skipBlockComment(s string, i int) int {
	i += 2
	for i+1 < len(s) {
		if s[i] == '*' && s[i+1] == '/' {
			return i + 2
		}
		i++
	}
	return len(s)
}

func parseDollar(s string, i int) (int, int) {
	j := i + 1
	n := 0
	for j < len(s) && s[j] >= '0' && s[j] <= '9' {
		n = n*10 + int(s[j]-'0')
		j++
	}
	return n, j
}

func readDollarTag(s string, i int) (string, int, bool) {
	if i >= len(s) || s[i] != '$' {
		return "", i, false
	}
	j := i + 1
	if j < len(s) && s[j] == '$' {
		return "$$", j + 1, true
	}
	if j >= len(s) || !identStart(s[j]) {
		return "", i, false
	}
	j++
	for j < len(s) && identPart(s[j]) {
		j++
	}
	if j >= len(s) || s[j] != '$' {
		return "", i, false
	}
	return s[i : j+1], j + 1, true
}

func skipDollarQuote(s string, i int, tag string) int {
	for i+len(tag) <= len(s) {
		if s[i:i+len(tag)] == tag {
			return i + len(tag)
		}
		i++
	}
	return len(s)
}

func skipSpaceAndComments(s string, i int) int {
	for i < len(s) {
		c := s[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			i++
			continue
		}
		if i+1 < len(s) && s[i] == '-' && s[i+1] == '-' {
			i = skipLineComment(s, i)
			continue
		}
		if s[i] == '#' {
			i = skipLineComment(s, i)
			continue
		}
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '*' {
			i = skipBlockComment(s, i)
			continue
		}
		return i
	}
	return i
}

func rewriteRawPlaceholders(fragment string, d Dialect, n int) (string, int, int, error) {
	var b strings.Builder
	count := 0
	i := 0
	for i < len(fragment) {
		if fragment[i] == '\'' {
			end := skipQuoted(fragment, i, '\'')
			b.WriteString(fragment[i:end])
			i = end
			continue
		}
		if fragment[i] == '"' {
			end := skipQuoted(fragment, i, '"')
			b.WriteString(fragment[i:end])
			i = end
			continue
		}
		if fragment[i] == '`' {
			end := skipQuoted(fragment, i, '`')
			b.WriteString(fragment[i:end])
			i = end
			continue
		}
		if fragment[i] == '?' {
			ph, next := placeholder(d, n)
			n = next
			b.WriteString(ph)
			count++
			i++
			continue
		}
		b.WriteByte(fragment[i])
		i++
	}
	return b.String(), n, count, nil
}
