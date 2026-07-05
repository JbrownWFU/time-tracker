package main

import (
	"database/sql"
	"fmt"
	"slices"

	_ "modernc.org/sqlite"
)

const (
	makeTables = `
	create table if not exists Jobs (
		id integer primary key,
		name text not null unique,
		desc text not null,
		created_at text default current_timestamp,
		status text not null
	);

	create table if not exists Spans (
		id integer primary key,
		job_id integer references Jobs(id),
		start_time text default current_timestamp,
		end_time text null,
		notes text
	);
	
	create table if not exists Notes (
		id integer primary key,
		entry_id integer references Spans(id),
		content text
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
	updateJob = `
	update Jobs set status = ? 
	where id = ?
	`

	writeSpan = `
	insert into Spans (job_id) 
	values (?)
	`
	getSpan = `
	select * from Spans
	where id = ?
	`
	getOpenSpanID = `
	select max(id) from Spans
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
	_, err := s.db.Exec(makeTables)
	if err != nil {
		return fmt.Errorf("make tables: %w", err)
	}

	return nil
}

// Job management -----

// Get job by ID
func (s *SqlConn) GetJob(id int) (Job, error) {
	var j Job
	err := s.db.QueryRow(getJob, id).Scan(&j.ID, &j.Name, &j.Desc, &j.Status)
	if err != nil {
		return Job{}, err
	}

	return j, nil
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

	// Read job back in and return
	id, err := res.LastInsertId()
	if err != nil {
		return Job{}, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	return s.GetJob(int(id))
}

// Update job status
func (s *SqlConn) UpdateJobStatus(id int, status string) (Job, error) {
	// Todo move to API layer - business logic
	if !slices.Contains(validStatuses, status) {
		return Job{}, fmt.Errorf("invalid status: %s", status)
	}

	_, err := s.db.Exec(updateJob, status, id)
	if err != nil {
		return Job{}, fmt.Errorf("update status failed: %w", err)
	}

	// Read back updated job
	// LastInsertId() only works on inserts
	j, err := s.GetJob(id)
	if err != nil {
		return Job{}, nil
	}

	return j, nil
}

// Time spans -----

// Write span with null endtime and return span ID
func (s *SqlConn) WriteSpan(jobId int, notes *string) (int, error) {
	res, err := s.db.Exec(writeSpan, jobId)
	if err != nil {
		return -1, fmt.Errorf("write span failed: %w", err)
	}

	// Read job back in and return
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	return int(id), nil
}

// Update span with endtime
