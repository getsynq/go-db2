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
	// One connection for every query, so a cursor left open by one of them
	// fails the next (SQLCODE -519).
	db.SetMaxOpenConns(1)
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

	// Db2 sends one row per query block when the rows carry XML.
	rows, err := db.QueryContext(ctx, `SELECT XMLELEMENT(NAME "t", TABNAME) FROM SYSCAT.TABLES FETCH FIRST 5 ROWS ONLY`)
	if err != nil {
		t.Fatalf("querying several XML rows: %v", err)
	}
	var xmlRows int
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scanning an XML row: %v", err)
		}
		if !strings.HasPrefix(v, "<t>") {
			t.Fatalf("XML row = %q", v)
		}
		xmlRows++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating XML rows: %v", err)
	}
	rows.Close()
	if xmlRows != 5 {
		t.Fatalf("got %d XML rows, want 5", xmlRows)
	}
}

// A result set many query blocks long, with rows split across blocks.
func TestLive_LargeResultSet(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	var want int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM SYSCAT.COLUMNS`).Scan(&want); err != nil {
		t.Fatalf("counting SYSCAT.COLUMNS: %v", err)
	}
	for _, args := range [][]any{nil, {"%"}} {
		q := `SELECT TABSCHEMA, TABNAME, COLNAME, TYPENAME, REMARKS FROM SYSCAT.COLUMNS`
		if args != nil {
			q += ` WHERE TABNAME LIKE ?` // the prepared-statement path
		}
		rows, err := db.QueryContext(ctx, q, args...)
		if err != nil {
			t.Fatalf("querying SYSCAT.COLUMNS (args %v): %v", args, err)
		}
		got := 0
		for rows.Next() {
			var schema, table, column, typ string
			var remarks sql.NullString
			if err := rows.Scan(&schema, &table, &column, &typ, &remarks); err != nil {
				t.Fatalf("scanning row %d: %v", got+1, err)
			}
			got++
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("iterating SYSCAT.COLUMNS: %v", err)
		}
		rows.Close()
		if got != want {
			t.Fatalf("got %d rows (args %v), want %d", got, args, want)
		}
	}
}
