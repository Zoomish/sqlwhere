package sqlwhere_test

import (
	"fmt"

	"github.com/Zoomish/sqlwhere"
)

func Example() {
	tenantID := "acme"
	status := "active"
	ids := []int{1, 2}

	q := sqlwhere.On(`SELECT id, name FROM products WHERE tenant_id = $1 AND deleted_at IS NULL`).
		Bind(tenantID).
		AndIf(status != "", sqlwhere.Eq("status", status)).
		AndIf(len(ids) > 0, sqlwhere.In("id", ids...)).
		Order(sqlwhere.Desc("created_at"), sqlwhere.Asc("id")).
		Limit(20)

	query, args, err := q.Build(sqlwhere.Postgres)
	if err != nil {
		panic(err)
	}
	fmt.Println(query)
	fmt.Println(args)
	// Output:
	// SELECT id, name FROM products WHERE tenant_id = $1 AND deleted_at IS NULL AND ("status" = $2 AND "id" IN ($3, $4)) ORDER BY "created_at" DESC, "id" ASC LIMIT $5
	// [acme active 1 2 20]
}

func ExampleQuery_AndIf() {
	q := "widget"
	query, args, err := sqlwhere.On("SELECT id, name FROM products").
		AndIf(q != "", sqlwhere.Like("name", "%"+q+"%")).
		Build(sqlwhere.Postgres)
	if err != nil {
		panic(err)
	}
	fmt.Println(query)
	fmt.Println(args)
	// Output:
	// SELECT id, name FROM products WHERE ("name" LIKE $1)
	// [%widget%]
}
