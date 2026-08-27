package sqliteprobe

import (
	"database/sql"
	_ "modernc.org/sqlite"
	"sync"
)

// CRUD opens a database, creates a table, writes one value, and reads it back.
func CRUD(path string) (string, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return "", err
	}
	defer db.Close()
	if _, err = db.Exec("PRAGMA journal_mode=WAL; PRAGMA synchronous=NORMAL; PRAGMA busy_timeout=5000; CREATE TABLE IF NOT EXISTS gate (id INTEGER PRIMARY KEY, value TEXT NOT NULL);"); err != nil {
		return "", err
	}
	if _, err = db.Exec("INSERT OR REPLACE INTO gate(id, value) VALUES(1, 'ok')"); err != nil {
		return "", err
	}
	var value string
	if err = db.QueryRow("SELECT value FROM gate WHERE id=1").Scan(&value); err != nil {
		return "", err
	}
	return value, nil
}

// ConcurrentWrites exercises one shared connection pool, matching the app's
// long-lived store rather than opening one database handle per goroutine.
func ConcurrentWrites(path string, count int) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err = db.Exec("PRAGMA journal_mode=WAL; PRAGMA synchronous=NORMAL; PRAGMA busy_timeout=5000; CREATE TABLE IF NOT EXISTS gate (id INTEGER PRIMARY KEY, value TEXT NOT NULL);"); err != nil {
		return err
	}
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, execErr := db.Exec("INSERT OR REPLACE INTO gate(id, value) VALUES(?, 'ok')", id+1)
			errs <- execErr
		}(i)
	}
	wg.Wait()
	close(errs)
	for execErr := range errs {
		if execErr != nil {
			return execErr
		}
	}
	return nil
}
