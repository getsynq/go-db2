package db2

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"
)

// Runs against the server in DB2_DSN, as CI does; skipped without it.
func TestLive_XMLAndLargeObjectValues(t *testing.T) {
	dsn := os.Getenv("DB2_DSN")
	if dsn == "" {
		t.Skip("DB2_DSN not set")
	}
	db, err := sql.Open("db2", dsn)
	if err != nil {
		t.Fatalf("sql.Open failed: %v", err)
	}
	defer db.Close()
	// Each value on its own connection: a query returning XML ends its first
	// query block without ENDQRYRM, and only one CNTQRY is sent, so the cursor
	// stays open and the next statement on that connection fails with -519.
	db.SetMaxIdleConns(0)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	var doc string
	var id int
	if err := db.QueryRowContext(ctx,
		`SELECT XMLPARSE(DOCUMENT '<root><item id="1">x</item></root>'), 7 FROM SYSIBM.SYSDUMMY1`).Scan(&doc, &id); err != nil {
		t.Fatalf("reading an XML value: %v", err)
	}
	if doc != `<root><item id="1">x</item></root>` || id != 7 {
		t.Fatalf("XML row = (%q, %d)", doc, id)
	}

	var null sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT CAST(NULL AS XML) FROM SYSIBM.SYSDUMMY1`).Scan(&null); err != nil {
		t.Fatalf("reading a NULL XML value: %v", err)
	}
	if null.Valid {
		t.Fatalf("NULL XML value read as %q", null.String)
	}

	// An XML value spread over several DSS continuation segments.
	var big string
	if err := db.QueryRowContext(ctx,
		`SELECT XMLELEMENT(NAME "r", XMLAGG(XMLELEMENT(NAME "c", COLNAME))) FROM SYSCAT.COLUMNS`).Scan(&big); err != nil {
		t.Fatalf("reading a large XML value: %v", err)
	}
	if len(big) < 100000 || !strings.HasPrefix(big, "<r><c>") || !strings.HasSuffix(big, "</c></r>") {
		t.Fatalf("large XML value is %d bytes: %.40q…", len(big), big)
	}

	var blob []byte
	if err := db.QueryRowContext(ctx,
		`SELECT BLOB(CAST(REPEAT('z', 30000) AS VARCHAR(30000) FOR BIT DATA)) || BLOB(CAST(REPEAT('y', 30000) AS VARCHAR(30000) FOR BIT DATA))
		 FROM SYSIBM.SYSDUMMY1`).Scan(&blob); err != nil {
		t.Fatalf("reading a 60000-byte BLOB: %v", err)
	}
	if want := strings.Repeat("z", 30000) + strings.Repeat("y", 30000); string(blob) != want {
		t.Fatalf("BLOB is %d bytes starting %q, want 60000", len(blob), blob[:min(8, len(blob))])
	}
}
