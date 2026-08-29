package sqlwhere

import "testing"

func TestAnalyzeSQL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		sql           string
		hasWhere      bool
		hasOrder      bool
		hasLimit      bool
		hasOffset     bool
		maxDollar     int
		questionN     int
		compound      bool
		insert        bool
		updateFrom    bool
		insertKeyword string
	}{
		{
			name:     "plain from",
			sql:      "SELECT id FROM t",
			hasWhere: false,
		},
		{
			name:     "outer where",
			sql:      "SELECT id FROM t WHERE deleted_at IS NULL",
			hasWhere: true,
		},
		{
			name:     "subquery where only",
			sql:      "SELECT id FROM (SELECT id FROM u WHERE active = true) AS x",
			hasWhere: false,
		},
		{
			name:      "in subquery plus outer where",
			sql:       "SELECT id FROM t WHERE id IN (SELECT id FROM u WHERE active = $1)",
			hasWhere:  true,
			maxDollar: 1,
		},
		{
			name:          "order by",
			sql:           "SELECT id FROM t ORDER BY id",
			hasOrder:      true,
			insertKeyword: "ORDER",
		},
		{
			name:          "where then order",
			sql:           "SELECT id FROM t WHERE x = 1 ORDER BY id",
			hasWhere:      true,
			hasOrder:      true,
			insertKeyword: "ORDER",
		},
		{
			name:          "limit",
			sql:           "SELECT id FROM t LIMIT 1",
			hasLimit:      true,
			insertKeyword: "LIMIT",
		},
		{
			name:          "offset",
			sql:           "SELECT id FROM t OFFSET 5",
			hasOffset:     true,
			insertKeyword: "OFFSET",
		},
		{
			name:      "placeholders",
			sql:       "SELECT id FROM t WHERE a = $1 AND b = $2",
			hasWhere:  true,
			maxDollar: 2,
		},
		{
			name:      "dollar in string ignored",
			sql:       "SELECT id FROM t WHERE name = '$1'",
			hasWhere:  true,
			maxDollar: 0,
		},
		{
			name:      "question marks",
			sql:       "SELECT id FROM t WHERE a = ? AND b = ?",
			hasWhere:  true,
			questionN: 2,
		},
		{
			name:     "select limit column before from",
			sql:      "SELECT limit FROM t",
			hasLimit: false,
			hasWhere: false,
		},
		{
			name:     "cte inner where",
			sql:      "WITH x AS (SELECT id FROM u WHERE active = true) SELECT id FROM x",
			hasWhere: false,
		},
		{
			name:      "dollar quoted body",
			sql:       "SELECT id FROM t WHERE body = $tag$ WHERE $1 $tag$",
			hasWhere:  true,
			maxDollar: 0,
		},
		{
			name:     "union",
			sql:      "SELECT a FROM t UNION SELECT b FROM u",
			compound: true,
		},
		{
			name:     "union inside subquery",
			sql:      "SELECT id FROM (SELECT a FROM t UNION SELECT b FROM u) AS x",
			compound: false,
		},
		{
			name:      "insert values",
			sql:       "INSERT INTO t (a) VALUES ($1)",
			insert:    true,
			maxDollar: 1,
		},
		{
			name:       "update from",
			sql:        "UPDATE t SET a = 1 FROM u WHERE t.id = u.id",
			updateFrom: true,
			hasWhere:   true,
		},
		{
			name:       "update without from",
			sql:        "UPDATE t SET a = 1 WHERE id = $1",
			updateFrom: false,
			hasWhere:   true,
			maxDollar:  1,
		},
		{
			name:       "update set from column",
			sql:        "UPDATE t SET from = 1 WHERE id = $1",
			updateFrom: false,
			hasWhere:   true,
			maxDollar:  1,
		},
		{
			name:          "group by",
			sql:           "SELECT id FROM t GROUP BY id",
			insertKeyword: "GROUP",
		},
		{
			name:          "having",
			sql:           "SELECT status FROM t GROUP BY status HAVING count(*) > 1",
			insertKeyword: "GROUP",
		},
		{
			name:          "returning",
			sql:           "UPDATE t SET a = 1 WHERE id = $1 RETURNING id",
			hasWhere:      true,
			maxDollar:     1,
			insertKeyword: "RETURNING",
		},
		{
			name:          "fetch first",
			sql:           "SELECT id FROM t FETCH FIRST 10 ROWS ONLY",
			insertKeyword: "FETCH",
		},
		{
			name:          "window",
			sql:           "SELECT id FROM t WINDOW w AS (ORDER BY id)",
			insertKeyword: "WINDOW",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := analyzeSQL(tt.sql)
			if got.hasOuterWhere != tt.hasWhere {
				t.Errorf("hasOuterWhere = %v, want %v", got.hasOuterWhere, tt.hasWhere)
			}
			if got.hasOuterOrder != tt.hasOrder {
				t.Errorf("hasOuterOrder = %v, want %v", got.hasOuterOrder, tt.hasOrder)
			}
			if got.hasOuterLimit != tt.hasLimit {
				t.Errorf("hasOuterLimit = %v, want %v", got.hasOuterLimit, tt.hasLimit)
			}
			if got.hasOuterOffset != tt.hasOffset {
				t.Errorf("hasOuterOffset = %v, want %v", got.hasOuterOffset, tt.hasOffset)
			}
			if got.maxDollar != tt.maxDollar {
				t.Errorf("maxDollar = %d, want %d", got.maxDollar, tt.maxDollar)
			}
			if got.questionN != tt.questionN {
				t.Errorf("questionN = %d, want %d", got.questionN, tt.questionN)
			}
			if got.compound != tt.compound {
				t.Errorf("compound = %v, want %v", got.compound, tt.compound)
			}
			if got.insert != tt.insert {
				t.Errorf("insert = %v, want %v", got.insert, tt.insert)
			}
			if got.updateFrom != tt.updateFrom {
				t.Errorf("updateFrom = %v, want %v", got.updateFrom, tt.updateFrom)
			}
			if tt.insertKeyword != "" {
				if got.insertAt >= len(tt.sql) || got.insertAt < 0 {
					t.Fatalf("insertAt = %d, sql len %d", got.insertAt, len(tt.sql))
				}
				rest := tt.sql[got.insertAt:]
				if len(rest) < len(tt.insertKeyword) || rest[:len(tt.insertKeyword)] != tt.insertKeyword {
					t.Errorf("insert at %q, want prefix %q", rest, tt.insertKeyword)
				}
			}
		})
	}
}
