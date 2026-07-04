package main

import (
	"database/sql"
	"fmt"
	"slices"

	_ "modernc.org/sqlite"
)

// Only allow one job at a time to be active
const (
	makeJobsTable = `
	create table if not exists Jobs (
		id integer primary key,
		name text not null unique,
		desc text,
		created_at text default current_timestamp,
		status text not null
	)
	`
	writeJob = `
	insert into Jobs (name, desc, status)
	values (?, ?, ?)
	`
	getJob = `
	select id, name, desc, status from Jobs
	where id = ?
	`
)

var validStatuses = []string{"todo", "active", "done"}

type SqlConn struct {
	db *sql.DB
}

// Connect to DB at path / create DB
func NewSqlConn(path string) (SqlConn, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return SqlConn{}, fmt.Errorf("open: %w", err)
	}

	// Ping to make sure we are connected
	if err = db.Ping(); err != nil {
		db.Close()
		return SqlConn{}, fmt.Errorf("ping: %w", err)
	}

	return SqlConn{db: db}, nil
}

func (s *SqlConn) Close() error {
	return s.db.Close()
}

func (s *SqlConn) MakeTables() error {
	_, err := s.db.Exec(makeJobsTable)
	if err != nil {
		return fmt.Errorf("make tables: %w", err)
	}
	
	return nil
}

// Write job to storage
func (s *SqlConn) WriteJob(name string, desc string, status string) (Job, error) {
	if !slices.Contains(validStatuses, status) {
		return Job{}, fmt.Errorf("invalid status: %s", status)
	}

	res, err := s.db.Exec(writeJob, name, desc, status)
	if err != nil {
		return Job{}, fmt.Errorf("write job insert failed: %w", err)
	}
	
	id, err := res.LastInsertId()
	if err != nil {
		return Job{}, fmt.Errorf("failed to get last insert ID: %w", err)
	}
	
	var j Job 
	err = s.db.QueryRow(getJob, id).Scan(&j.ID, &j.Name, &j.Desc, &j.Status)
	if err != nil {
		return Job{}, fmt.Errorf("failed to return last inserted job: %w", err)
	}
	
	return j, nil
}

func (s *SqlConn) FindActiveJob() (Job, error) {
	fmt.Println("todo")
	return Job{}, nil
}