package db2

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"
)

// Runs against the server in DB2_DSN, as CI does; skipped without it.
// An aggregate over a NULL makes Db2 attach SQLSTATE 01003 (null values
// eliminated) to the row, which then starts with a full SQLCA.
func TestLive_RowWithWarning(t *testing.T) {
	dsn := os.Getenv("DB2_DSN")
	if dsn == "" {
		t.Skip("DB2_DSN not set")
	}
	db, err := sql.Open("db2", dsn)
	if err != nil {
		t.Fatalf("sql.Open failed: %v", err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	var n int
	var avg float64
	err = db.QueryRowContext(ctx,
		`SELECT COUNT(*), CAST(AVG(x) AS DOUBLE) FROM (VALUES (1), (CAST(NULL AS INTEGER)), (3)) AS t(x)`).Scan(&n, &avg)
	if err != nil {
		t.Fatalf("reading an aggregate row with a warning: %v", err)
	}
	if n != 3 || avg != 2 {
		t.Fatalf("got count=%d avg=%v, want 3 and 2", n, avg)
	}
}
