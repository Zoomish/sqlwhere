package sqlwhere

import "testing"

func BenchmarkBuild(b *testing.B) {
	ids := []int{1, 2, 3, 4, 5}
	status := "active"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := On(`SELECT id, name FROM products WHERE tenant_id = $1 AND deleted_at IS NULL`).
			Bind("acme").
			AndIf(status != "", Eq("status", status)).
			AndIf(len(ids) > 0, In("id", ids...)).
			Order(Desc("created_at"), Asc("id")).
			Limit(20).
			Build(Postgres)
		if err != nil {
			b.Fatal(err)
		}
	}
}
