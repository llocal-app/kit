package kit

import (
	"context"
	"database/sql"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"
)

// A hundred requests at once, reading, writing, writing with RETURNING, through a prepared
// statement and in transactions, as a busy app's are: none may wait out the busy timeout,
// here a second, and fail on the lock.
func TestBusyDatabase(t *testing.T) {
	defer func(d time.Duration) { busyTimeout = d }(busyTimeout)
	busyTimeout = time.Second
	db, err := openDB(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, body TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	insert, err := db.Prepare(`INSERT INTO items (body) VALUES (?)`)
	if err != nil {
		t.Fatal(err)
	}
	ops := []func() error{
		func() error {
			_, err := db.Exec(`INSERT INTO items (body) VALUES ('x')`)
			return err
		},
		func() error {
			_, err := insert.Exec("p")
			return err
		},
		func() error {
			var n int
			return db.QueryRow(`SELECT count(*) FROM items`).Scan(&n)
		},
		func() error {
			var id int
			return db.QueryRow(`INSERT INTO items (body) VALUES ('y') RETURNING id`).Scan(&id)
		},
		func() error {
			tx, err := db.Begin()
			if err != nil {
				return err
			}
			defer tx.Rollback()
			var n int
			if err := tx.QueryRow(`SELECT count(*) FROM items`).Scan(&n); err != nil {
				return err
			}
			if _, err := tx.Exec(`UPDATE items SET body = 'z' WHERE id = ?`, n); err != nil {
				return err
			}
			return tx.Commit()
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var mu sync.Mutex
	var waits []time.Duration
	var failed error
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				start := time.Now()
				err := ops[i%len(ops)]()
				mu.Lock()
				waits = append(waits, time.Since(start))
				if err != nil && err != sql.ErrNoRows {
					failed = err
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	slices.Sort(waits)
	t.Logf("%d statements, slowest 1%% over %v, slowest %v", len(waits), waits[len(waits)*99/100], waits[len(waits)-1])
	if failed != nil {
		t.Fatal(failed)
	}
}
