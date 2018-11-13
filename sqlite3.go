package sqlite3

/*

#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include "sqlite3.h"

int sqlite3_open_x(const char *zURI, int nBytes, sqlite3 **ppDb) {
	char cURI[512];
	if (nBytes > sizeof(cURI)) {
		return SQLITE_ERROR;
	}
	memcpy(cURI, zURI, nBytes);
	cURI[nBytes] = 0;
	return sqlite3_open(cURI, ppDb);
}

int sqlite3_bind_text_x(sqlite3_stmt *pStmt, int iCol, const char *zSQL, int nBytes) {
	return sqlite3_bind_text(pStmt, iCol, zSQL, nBytes, SQLITE_STATIC);
}

int sqlite3_copy(sqlite3 *pDb, const char *zURI, int nBytes, int isSave) {
	sqlite3 *pTo;
	sqlite3 *pFrom;
	sqlite3 *pTemp;
	int rc = sqlite3_open_x(zURI, nBytes, &pTemp);
	if (rc != SQLITE_OK) {
		sqlite3_close_v2(pTemp);
		return rc;
	}
	pTo = (isSave ? pTemp : pDb);
	pFrom = (isSave ? pDb : pTemp);
	sqlite3_backup *pBackup = sqlite3_backup_init(pTo, "main", pFrom, "main");
	if (pBackup) {
		sqlite3_backup_step(pBackup, -1);
		sqlite3_backup_finish(pBackup);
	}
	rc = sqlite3_errcode(pTo);
	sqlite3_close_v2(pTemp);
	return rc;
}

int writeMem(void *b, int n, const void *v, int s) {
	if (n < s) { return 0; }
	memcpy(b, v, s);
	return s;
}

int writeInt8(void *b, int n, int8_t v) {
	return writeMem(b, n, &v, sizeof(v));
}

int writeInt32(void *b, int n, int32_t v) {
	return writeMem(b, n, &v, sizeof(v));
}

int writeInt64(void *b, int n, int64_t v) {
	return writeMem(b, n, &v, sizeof(v));
}

int sqlite3_write_int64(sqlite3_stmt *pStmt, int iCol, uint8_t *pBuf, int nBytes) {
	int a = writeInt8(pBuf, nBytes, SQLITE_INTEGER);
	if (a == 0) { return 0; }
	int b = writeInt64(pBuf + a, nBytes - a, 0);
	if (b == 0) { return 0; }
	return a + b;
}

int sqlite3_write_text(sqlite3_stmt *pStmt, int iCol, uint8_t *pBuf, int nBytes) {
	int a = writeInt8(pBuf, nBytes, SQLITE_TEXT);
	if (a == 0) { return 0; }
	int b = writeInt32(pBuf + a, nBytes - a, 0);
	if (b == 0) { return 0; }
	int c = writeMem(pBuf + (a + b), nBytes - (a + b), "text", 4);
	if (c == 0) { return 0; }
	return a + b + c;
}

int sqlite3_write_null(sqlite3_stmt *pStmt, int iCol, uint8_t *pBuf, int nBytes) {
	return writeInt8(pBuf, nBytes, SQLITE_NULL);
}

*/
import "C"

import (
	"errors"
	"reflect"
	"sync"
	"unsafe"
)

var byteBufPool = sync.Pool{New: func() interface{} {
	return make([]byte, 8192)
}}

type DB struct {
	p *C.sqlite3
}

type Stmt struct {
	p *C.sqlite3_stmt
	b []byte
	r []byte
}

func cStr(s string) (*C.char, C.int) {
	h := (*reflect.StringHeader)(unsafe.Pointer(&s))
	return (*C.char)(unsafe.Pointer(h.Data)), C.int(h.Len)
}

func Open(URI string) (*DB, error) {
	var p *C.sqlite3
	z, n := cStr(URI)
	r := C.sqlite3_open_x(z, n, &p)
	if r != C.SQLITE_OK {
		C.sqlite3_close_v2(p)
		return nil, errors.New("cannot open database")
	}
	return &DB{p: p}, nil
}

func (d *DB) Close() {
	C.sqlite3_close_v2(d.p)
}

func (d *DB) Backup(URI string) error {
	z, n := cStr(URI)
	r := C.sqlite3_copy(d.p, z, n, 1)
	if r != C.SQLITE_OK {
		return errors.New("cannot backup database")
	}
	return nil
}

func (d *DB) Restore(URI string) error {
	z, n := cStr(URI)
	r := C.sqlite3_copy(d.p, z, n, 0)
	if r != C.SQLITE_OK {
		return errors.New("cannot restore database")
	}
	return nil
}

func (d *DB) Prepare(SQL string) (*Stmt, error) {
	var p *C.sqlite3_stmt
	z, n := cStr(SQL)
	r := C.sqlite3_prepare_v2(d.p, z, n, &p, nil)
	if r != C.SQLITE_OK {
		return nil, errors.New("cannot prepare statement")
	}
	return &Stmt{p: p}, nil
}

func (s *Stmt) Close() {
	if s.b != nil {
		byteBufPool.Put(s.b)
		s.b = nil
		s.r = nil
	}
	C.sqlite3_finalize(s.p)
}

func (s *Stmt) bind(args ...interface{}) error {
	for k, v := range args {
		i := C.int(k + 1)
		r := C.int(0)
		switch v := v.(type) {
		case int:
			r = C.sqlite3_bind_int64(s.p, i, C.sqlite3_int64(v))
		case string:
			z, n := cStr(v)
			r = C.sqlite3_bind_text_x(s.p, i, z, n)
		default:
			return errors.New("cannot bind parameters")
		}
		if r != C.SQLITE_OK {
			return errors.New("cannot bind parameters")
		}
	}
	return nil
}

func (s *Stmt) Exec(args ...interface{}) error {
	r := C.sqlite3_reset(s.p)
	if r != C.SQLITE_OK {
		return errors.New("cannot reset statement")
	}
	err := s.bind(args...)
	if err != nil {
		return err
	}
	r = C.sqlite3_step(s.p)
	if r == C.SQLITE_DONE {
		return nil
	}
	if r == C.SQLITE_ROW {
		return nil
	}
	return errors.New("cannot execute statement")
}

func (s *Stmt) prefetch() {
	if s.b == nil {
		s.b = byteBufPool.Get().([]byte)
	}
	s.r = s.b[:0]
}
