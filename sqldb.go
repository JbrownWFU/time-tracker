package main

import (
	_ "modernc.org/sqlite"
	"database/sql"
)

type SqlConn struct {
	db *sql.DB
}

// Connect to DB at path / create DB
func NewSqlConn(path string) (SqlConn, error) {
	
}