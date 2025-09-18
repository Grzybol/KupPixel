package sqlite3

/*
#cgo LDFLAGS: -lsqlite3
#include <sqlite3.h>
#include <stdlib.h>

static const char* gosqlite_errmsg(sqlite3* db) {
        return sqlite3_errmsg(db);
}
*/
import "C"

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"unsafe"
)

func init() {
	sql.Register("sqlite3", &Driver{})
}

type Driver struct{}

type conn struct {
	db *C.sqlite3
}

type result struct {
	lastID       int64
	rowsAffected int64
}

type rows struct {
	columns []string
	data    [][]driver.Value
	index   int
}

type tx struct {
	c    *conn
	done bool
}

func (d *Driver) Open(name string) (driver.Conn, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	var db *C.sqlite3
	if rc := C.sqlite3_open(cName, &db); rc != C.SQLITE_OK {
		err := errors.New(C.GoString(C.gosqlite_errmsg(db)))
		if db != nil {
			C.sqlite3_close(db)
		}
		return nil, err
	}

	return &conn{db: db}, nil
}

func (c *conn) Prepare(query string) (driver.Stmt, error) {
	return nil, errors.New("prepared statements are not supported")
}

func (c *conn) Close() error {
	if c.db == nil {
		return nil
	}
	if rc := C.sqlite3_close(c.db); rc != C.SQLITE_OK {
		return errors.New(C.GoString(C.gosqlite_errmsg(c.db)))
	}
	c.db = nil
	return nil
}

func (c *conn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *conn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if err := c.execSimple("BEGIN TRANSACTION"); err != nil {
		return nil, err
	}
	return &tx{c: c}, nil
}

func (c *conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if len(args) > 0 {
		return nil, errors.New("query parameters are not supported")
	}
	return c.exec(query)
}

func (c *conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if len(args) > 0 {
		return nil, errors.New("query parameters are not supported")
	}
	return c.query(query)
}

func (c *conn) exec(query string) (driver.Result, error) {
	cQuery := C.CString(query)
	defer C.free(unsafe.Pointer(cQuery))

	var errMsg *C.char
	rc := C.sqlite3_exec(c.db, cQuery, nil, nil, &errMsg)
	if rc != C.SQLITE_OK {
		defer C.sqlite3_free(unsafe.Pointer(errMsg))
		return nil, errors.New(C.GoString(errMsg))
	}

	res := result{
		lastID:       int64(C.sqlite3_last_insert_rowid(c.db)),
		rowsAffected: int64(C.sqlite3_changes(c.db)),
	}
	return res, nil
}

func (c *conn) query(query string) (driver.Rows, error) {
	cQuery := C.CString(query)
	defer C.free(unsafe.Pointer(cQuery))

	var stmt *C.sqlite3_stmt
	if rc := C.sqlite3_prepare_v2(c.db, cQuery, -1, &stmt, nil); rc != C.SQLITE_OK {
		return nil, errors.New(C.GoString(C.gosqlite_errmsg(c.db)))
	}
	defer C.sqlite3_finalize(stmt)

	columnCount := int(C.sqlite3_column_count(stmt))
	columns := make([]string, columnCount)
	for i := 0; i < columnCount; i++ {
		columns[i] = C.GoString(C.sqlite3_column_name(stmt, C.int(i)))
	}

	data := make([][]driver.Value, 0)
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_ROW {
			row := make([]driver.Value, columnCount)
			for i := 0; i < columnCount; i++ {
				switch C.sqlite3_column_type(stmt, C.int(i)) {
				case C.SQLITE_INTEGER:
					row[i] = int64(C.sqlite3_column_int64(stmt, C.int(i)))
				case C.SQLITE_FLOAT:
					row[i] = float64(C.sqlite3_column_double(stmt, C.int(i)))
				case C.SQLITE_TEXT, C.SQLITE_BLOB:
					text := C.sqlite3_column_text(stmt, C.int(i))
					if text != nil {
						row[i] = C.GoString((*C.char)(unsafe.Pointer(text)))
					} else {
						row[i] = ""
					}
				case C.SQLITE_NULL:
					row[i] = nil
				default:
					row[i] = nil
				}
			}
			data = append(data, row)
		} else if rc == C.SQLITE_DONE {
			break
		} else {
			return nil, errors.New(C.GoString(C.gosqlite_errmsg(c.db)))
		}
	}

	return &rows{columns: columns, data: data}, nil
}

func (c *conn) execSimple(query string) error {
	_, err := c.exec(query)
	return err
}

func (r result) LastInsertId() (int64, error) {
	return r.lastID, nil
}

func (r result) RowsAffected() (int64, error) {
	return r.rowsAffected, nil
}

func (r *rows) Columns() []string {
	return r.columns
}

func (r *rows) Close() error {
	r.data = nil
	return nil
}

func (r *rows) Next(dest []driver.Value) error {
	if r.index >= len(r.data) {
		return io.EOF
	}
	copy(dest, r.data[r.index])
	r.index++
	return nil
}

func (t *tx) Commit() error {
	if t.done {
		return errors.New("transaction already completed")
	}
	t.done = true
	return t.c.execSimple("COMMIT")
}

func (t *tx) Rollback() error {
	if t.done {
		return errors.New("transaction already completed")
	}
	t.done = true
	return t.c.execSimple("ROLLBACK")
}
