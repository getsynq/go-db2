package db2

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// inListQuery returns a query of at least minBytes whose single row counts
// the IN list values that match, which is always one.
func inListQuery(minBytes int) string {
	var b strings.Builder
	b.WriteString("SELECT COUNT(*) FROM SYSIBM.SYSDUMMY1 WHERE 1 IN (1")
	for i := 2; b.Len() < minBytes; i++ {
		fmt.Fprintf(&b, ", %d", i)
	}
	b.WriteString(")")
	return b.String()
}

// Runs against the server in DB2_DSN, as CI does; skipped without it.
func TestLive_LargeStatements(t *testing.T) {
	dsn := os.Getenv("DB2_DSN")
	if dsn == "" {
		t.Skip("DB2_DSN not set")
	}
	db, err := sql.Open("db2", dsn)
	if err != nil {
		t.Fatalf("sql.Open failed: %v", err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	for _, size := range []int{16 << 10, 32 << 10, 64 << 10, 128 << 10, 1 << 20} {
		query := inListQuery(size)
		t.Run(fmt.Sprintf("query_%dKB", size>>10), func(t *testing.T) {
			var n int
			if err := db.QueryRowContext(ctx, query).Scan(&n); err != nil {
				t.Fatalf("%d byte query: %v", len(query), err)
			}
			if n != 1 {
				t.Fatalf("%d byte query returned %d, want 1", len(query), n)
			}
		})
		t.Run(fmt.Sprintf("query_with_arg_%dKB", size>>10), func(t *testing.T) {
			var n int
			if err := db.QueryRowContext(ctx, query+" AND 1 = ?", 1).Scan(&n); err != nil {
				t.Fatalf("%d byte query: %v", len(query), err)
			}
			if n != 1 {
				t.Fatalf("%d byte query returned %d, want 1", len(query), n)
			}
		})
		t.Run(fmt.Sprintf("exec_%dKB", size>>10), func(t *testing.T) {
			stmt := "SET CURRENT QUERY OPTIMIZATION = 5" + strings.Repeat(" ", size)
			if _, err := db.ExecContext(ctx, stmt); err != nil {
				t.Fatalf("%d byte statement: %v", len(stmt), err)
			}
		})
	}
}
