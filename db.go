package kit

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sync/atomic"
	"time"

	"modernc.org/sqlite"
)

// busyTimeout is how long a write waits for its turn before it fails.
var busyTimeout = 5 * time.Second

func openDB(path string) (*sql.DB, error) {
	q := url.Values{}
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", busyTimeout.Milliseconds()))
	// Write transactions take the lock at BEGIN, so concurrent writers wait out busy_timeout
	// instead of failing with SQLITE_BUSY when a read lock can't be upgraded.
	q.Add("_txlock", "immediate")
	db := sql.OpenDB(connector{path + "?" + q.Encode(), make(writer, 1)})
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return db, nil
}

// writer lets one write at a time reach SQLite, in the order they came. SQLite allows one
// writer and makes the others poll for the lock, so under load an unlucky writer waits out
// busy_timeout and fails; queued here, every writer waits only its turn, and reads go on
// beside them.
type writer chan struct{}

func (w writer) lock(ctx context.Context) error {
	select {
	case w <- struct{}{}:
		return nil
	default:
	}
	t := time.NewTimer(busyTimeout)
	defer t.Stop()
	select {
	case w <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return fmt.Errorf("database is locked: another write held it for %s", busyTimeout)
	}
}

// turns counts the writer's turns taken: by writes, even those that change nothing, and by
// transactions. /healthz tells llocal's checker, which fails a request that only reads but
// takes one.
var turns atomic.Int64

func (w writer) unlock() {
	turns.Add(1)
	<-w
}

type connector struct {
	dsn string
	w   writer
}

func (c connector) Connect(context.Context) (driver.Conn, error) {
	conn, err := c.Driver().Open(c.dsn)
	if err != nil {
		return nil, err
	}
	return &liteConn{conn.(sqliteConn), c.w, false}, nil
}

func (connector) Driver() driver.Driver { return &sqlite.Driver{} }

type sqliteConn interface {
	driver.Conn
	driver.ConnBeginTx
	driver.ConnPrepareContext
	driver.ExecerContext
	driver.QueryerContext
	driver.Pinger
	driver.SessionResetter
	driver.Validator
}

// liteConn takes the writer's turn for each transaction, each statement run outside one,
// and each query that writes (RETURNING) until its rows are closed, prepared or not.
type liteConn struct {
	sqliteConn
	w  writer
	tx bool
}

func (c *liteConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if err := c.w.lock(ctx); err != nil {
		return nil, err
	}
	t, err := c.sqliteConn.BeginTx(ctx, opts)
	if err != nil {
		c.w.unlock()
		return nil, err
	}
	c.tx = true
	return liteTx{t, c}, nil
}

func (c *liteConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return c.exec(ctx, func() (driver.Result, error) { return c.sqliteConn.ExecContext(ctx, query, args) })
}

func (c *liteConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return c.query(ctx, returning.MatchString(query), func() (driver.Rows, error) { return c.sqliteConn.QueryContext(ctx, query, args) })
}

func (c *liteConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	s, err := c.sqliteConn.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	return liteStmt{s.(sqliteStmt), c, returning.MatchString(query)}, nil
}

func (c *liteConn) exec(ctx context.Context, run func() (driver.Result, error)) (driver.Result, error) {
	if !c.tx {
		if err := c.w.lock(ctx); err != nil {
			return nil, err
		}
		defer c.w.unlock()
	}
	return run()
}

var returning = regexp.MustCompile(`(?i)\breturning\b`)

func (c *liteConn) query(ctx context.Context, writes bool, run func() (driver.Rows, error)) (driver.Rows, error) {
	if c.tx || !writes {
		return run()
	}
	if err := c.w.lock(ctx); err != nil {
		return nil, err
	}
	rows, err := run()
	if err != nil {
		c.w.unlock()
		return nil, err
	}
	return liteRows{rows, c.w}, nil
}

type sqliteStmt interface {
	driver.Stmt
	driver.StmtExecContext
	driver.StmtQueryContext
}

type liteStmt struct {
	sqliteStmt
	c      *liteConn
	writes bool
}

func (s liteStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	return s.c.exec(ctx, func() (driver.Result, error) { return s.sqliteStmt.ExecContext(ctx, args) })
}

func (s liteStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	return s.c.query(ctx, s.writes, func() (driver.Rows, error) { return s.sqliteStmt.QueryContext(ctx, args) })
}

type liteTx struct {
	driver.Tx
	c *liteConn
}

func (t liteTx) Commit() error {
	defer t.end()
	return t.Tx.Commit()
}

func (t liteTx) Rollback() error {
	defer t.end()
	return t.Tx.Rollback()
}

func (t liteTx) end() {
	t.c.tx = false
	t.c.w.unlock()
}

type liteRows struct {
	driver.Rows
	w writer
}

func (r liteRows) Close() error {
	defer r.w.unlock()
	return r.Rows.Close()
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
