package sqliteprobe

import (
	"database/sql"
	_ "embed"
	"sync"

	_ "modernc.org/sqlite"
)

// SQL 语句外置在 queries/*.sql（S4 字符串代码歼灭）：内容与原内联字符串逐字
// 一致，两条 INSERT 语义不同（字面量 vs 参数化）分文件维护，不得合并。
//
//go:embed queries/schema.sql
var schemaSQL string

//go:embed queries/upsert_fixed.sql
var upsertFixedSQL string

//go:embed queries/upsert_param.sql
var upsertParamSQL string

//go:embed queries/select.sql
var selectSQL string

// CRUD opens a database, creates a table, writes one value, and reads it back.
func CRUD(path string) (string, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return "", err
	}
	defer db.Close()
	if _, err = db.Exec(schemaSQL); err != nil {
		return "", err
	}
	if _, err = db.Exec(upsertFixedSQL); err != nil {
		return "", err
	}
	var value string
	if err = db.QueryRow(selectSQL).Scan(&value); err != nil {
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
	if _, err = db.Exec(schemaSQL); err != nil {
		return err
	}
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, execErr := db.Exec(upsertParamSQL, id+1)
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
