package sqlite3

/*
#include <string.h>
#include "sqlite3.h"

#define URI_MAX_SIZE 256

int _sqlite3_open(_GoString_ URI, sqlite3 **ppDb) {
	size_t size = _GoStringLen(URI);
	if (size >= URI_MAX_SIZE) {
		return SQLITE_ERROR;
	}
	char cURI[URI_MAX_SIZE];
	memcpy(cURI, _GoStringPtr(URI), size);
	cURI[size] = 0;
	return sqlite3_open(cURI, ppDb);
}

int _sqlite3_prepare(sqlite3 *pDb, _GoString_ SQL, sqlite3_stmt **ppStmt) {
	return sqlite3_prepare_v2(pDb, _GoStringPtr(SQL), _GoStringLen(SQL), ppStmt, NULL);
}

int _sqlite3_bind_text_static(sqlite3_stmt *pStmt, int i, _GoString_ data) {
	sqlite3_bind_text(pStmt, i, _GoStringPtr(data), _GoStringLen(data), SQLITE_STATIC);
}
*/
import "C"

import (
	"errors"
)

type DB struct {
	p *C.sqlite3
}

type Stmt struct {
	p *C.sqlite3_stmt
}

const (
	SQLITE_OK   = C.SQLITE_OK
	SQLITE_DONE = C.SQLITE_DONE
	SQLITE_ROW  = C.SQLITE_ROW
)

func NewDB(URI string) (*DB, error) {
	var p *C.sqlite3
	r := C._sqlite3_open(URI, &p)
	if r != SQLITE_OK {
		C.sqlite3_close_v2(p)
		return nil, errors.New("cannot open database")
	}
	C.sqlite3_close_v2(nil)
	return &DB{p: p}, nil
}

func (d *DB) Close() {
	C.sqlite3_close_v2(d.p)
}

func (d *DB) Prepare(SQL string) (*Stmt, error) {
	var p *C.sqlite3_stmt
	r := C._sqlite3_prepare(d.p, SQL, &p)
	if r != SQLITE_OK {
		C.sqlite3_finalize(p)
		return nil, errors.New("cannot prepare statement")
	}
	return &Stmt{p: p}, nil
}

func (s *Stmt) Bind(args ...interface{}) error {
	for k, v := range args {
		i := C.int(k + 1)
		r := C.int(0)
		switch v := v.(type) {
		case int:
			r = C.sqlite3_bind_int(s.p, i, C.int(v))
		case string:
			r = C._sqlite3_bind_text_static(s.p, i, v)
		default:
			return errors.New("cannot bind parameters")
		}
		if r != SQLITE_OK {
			return errors.New("cannot bind parameters")
		}
	}
	return nil
}

func (s *Stmt) Exec(args ...interface{}) error {
	r := C.sqlite3_reset(s.p)
	if r != SQLITE_OK {
		return errors.New("cannot reset statement")
	}
	err := s.Bind(args...)
	if err != nil {
		return err
	}
	r = C.sqlite3_step(s.p)
	if r == SQLITE_DONE {
		return nil
	}
	if r == SQLITE_ROW {
		return nil
	}
	return errors.New("cannot execute statement")
}

func (s *Stmt) Close() {
	C.sqlite3_finalize(s.p)
}