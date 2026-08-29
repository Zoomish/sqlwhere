package itest

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Zoomish/sqlwhere"
)

type ListFilter struct {
	TenantID string
	Status   string
	MinN     *int
	IDs      []int
	Limit    int
}

func ListProductIDs(ctx context.Context, db *sql.DB, d sqlwhere.Dialect, f ListFilter) ([]int, error) {
	q := sqlwhere.On(listBase(d)).
		Bind(f.TenantID).
		AndIf(f.Status != "", sqlwhere.Eq("status", f.Status)).
		AndIfFn(f.MinN != nil, func() sqlwhere.Predicate {
			return sqlwhere.Gte("n", *f.MinN)
		}).
		AndIf(len(f.IDs) > 0, sqlwhere.In("id", f.IDs...)).
		Order(sqlwhere.Asc("id"))
	if f.Limit > 0 {
		q = q.Limit(f.Limit)
	}
	query, args, err := q.Build(d)
	if err != nil {
		return nil, fmt.Errorf("list product ids: build: %w", err)
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list product ids: query: %w", err)
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("list product ids: scan: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list product ids: rows: %w", err)
	}
	return ids, nil
}

func listBase(d sqlwhere.Dialect) string {
	if d == sqlwhere.Question {
		return `SELECT id FROM products WHERE tenant_id = ? AND deleted_at IS NULL`
	}
	return `SELECT id FROM products WHERE tenant_id = $1 AND deleted_at IS NULL`
}
