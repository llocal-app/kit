package kit

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"

	_ "modernc.org/sqlite"
)

func openDB(path string) (*sql.DB, error) {
	q := url.Values{}
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "busy_timeout(5000)")
	// Write transactions take the lock at BEGIN, so concurrent writers wait out busy_timeout
	// instead of failing with SQLITE_BUSY when a read lock can't be upgraded.
	q.Add("_txlock", "immediate")
	db, err := sql.Open("sqlite", path+"?"+q.Encode())
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return db, nil
}

// migrate applies each .sql file in dir, in name order, once.
func migrate(db *sql.DB, dir string) error {
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	// Off so a migration can rebuild a table that others reference; back on when done.
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS _migrations (name TEXT PRIMARY KEY)"); err != nil {
		return err
	}
	names, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return err
	}
	slices.Sort(names)
	applied := false
	for _, file := range names {
		name := filepath.Base(file)
		var done int
		if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM _migrations WHERE name = ?", name).Scan(&done); err != nil {
			return err
		}
		if done > 0 {
			continue
		}
		body, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO _migrations (name) VALUES (?)", name); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		applied = true
	}
	// Foreign keys are on at all other times, so only a migration can break a reference.
	if applied {
		var table string
		err := conn.QueryRowContext(ctx, `SELECT "table" FROM pragma_foreign_key_check`).Scan(&table)
		if err == nil {
			return fmt.Errorf("migrations left rows in %s that point at rows that do not exist", table)
		}
		if err != sql.ErrNoRows {
			return err
		}
	}
	_, err = conn.ExecContext(ctx, "PRAGMA foreign_keys = ON")
	return err
}
