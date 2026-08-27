package sqlitegate

import "github.com/axuitomo/CFST-GUI/internal/sqliteprobe"

// CRUD validates the pure-Go SQLite driver through gomobile.
func CRUD(path string) (string, error) { return sqliteprobe.CRUD(path) }

// ConcurrentWrites validates shared-pool writes through gomobile.
func ConcurrentWrites(path string, count int) error { return sqliteprobe.ConcurrentWrites(path, count) }
