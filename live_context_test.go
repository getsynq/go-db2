package db2

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"
)

// Runs against the server in DB2_DSN, as CI does; skipped without it.
func TestLive_QueryHonoursContextDeadline(t *testing.T) {
	dsn := os.Getenv("DB2_DSN")
	if dsn == "" {
		t.Skip("DB2_DSN not set")
	}
	db, err := sql.Open("db2", dsn)
	if err != nil {
		t.Fatalf("sql.Open failed: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// A cross join large enough to run for minutes.
	done := make(chan error, 1)
	start := time.Now()
	go func() {
		var n int64
		done <- db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM SYSCAT.COLUMNS a, SYSCAT.COLUMNS b, SYSCAT.COLUMNS c").Scan(&n)
	}()

	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected context.DeadlineExceeded, got: %v", err)
		}
		if elapsed := time.Since(start); elapsed > 10*time.Second {
			t.Fatalf("query returned %s after its 2s deadline", elapsed)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("query did not return within 30s of a 2s deadline")
	}

	var one int
	if err := db.QueryRowContext(context.Background(), "SELECT 1 FROM SYSIBM.SYSDUMMY1").Scan(&one); err != nil || one != 1 {
		t.Fatalf("query after the interrupted one: one=%d err=%v", one, err)
	}
}
