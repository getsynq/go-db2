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

// Runs against the server in DB2_DSN, as CI does; skipped without it.
func TestLive_LargeParameters(t *testing.T) {
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

	// A string parameter goes out as UTF-16 of at most 16383 characters, so a
	// parameter payload past one DSS takes several of them.
	for _, params := range []int{1, 2, 4} {
		t.Run(fmt.Sprintf("%d_params_of_16000_chars", params), func(t *testing.T) {
			var args []any
			var sums []string
			for i := range params {
				args = append(args, strings.Repeat(string(rune('a'+i)), 16000))
				sums = append(sums, "LENGTH(CAST(? AS VARCHAR(16000)))")
			}
			var n int
			query := "SELECT " + strings.Join(sums, " + ") + " FROM SYSIBM.SYSDUMMY1"
			if err := db.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
				t.Fatalf("%d parameters: %v", params, err)
			}
			if n != 16000*params {
				t.Fatalf("%d parameters summed to %d bytes, want %d", params, n, 16000*params)
			}
		})
	}
}
