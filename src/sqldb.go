package main

import (
	"database/sql"
	"fmt"
	"slices"
	"time"

	_ "modernc.org/sqlite"
)

// sqlTimeFormat matches sqlite's current_timestamp text format so timestamps
// generated in Go sort and parse the same as the values it used to default to.
const sqlTimeFormat = "2006-01-02 15:04:05"

const (
	makeTables = `
	create table if not exists Jobs (
		id integer primary key,
		name text not null unique,
		desc text not null,
		created_at text not null,
		status text not null
	);

	create table if not exists Spans (
		id integer primary key,
		job_id integer references Jobs(id),
		start_time text not null,
		end_time text null
	);

	create table if not exists Notes (
		id integer primary key,
		entry_id integer references Spans(id),
		content text
	)
	`
	writeJob = `
	insert into Jobs (name, desc, status, created_at)
	values (?, ?, ?, ?)
	`
	getJob = `
	select id, name, desc, status from Jobs
	where id = ?
	`
	updateJob = `
	update Jobs set status = ? 
	where id = ?
	`
	resolveJob = `
	select id from Jobs
	where name = ?
	`

	writeSpan = `
	insert into Spans (job_id, start_time)
	values (?, ?)
	`
	getSpan = `
	select * from Spans
	where id = ?
	`
	getOpenSpanID = `
	select max(id) from Spans
	where end_time is null
	`
	updateSpan = `
	update Spans set end_time = ?
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

func (s *SqlConn) ResolveJob(name string) (int, error) {
	var id int
	err := s.db.QueryRow(resolveJob, name).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to resolve job by name: %w", err)
	}

	return id, nil
}

// Write job to storage
func (s *SqlConn) WriteJob(name string, desc string, status string) (int, error) {
	if !slices.Contains(validStatuses, status) {
		return 0, fmt.Errorf("invalid status: %s", status)
	}

	res, err := s.db.Exec(writeJob, name, desc, status, time.Now().UTC().Format(sqlTimeFormat))
	if err != nil {
		return 0, fmt.Errorf("write job insert failed: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	return int(id), nil
}

// Update job status
func (s *SqlConn) UpdateJobStatus(id int, status string) (int, error) {
	// Todo move to API layer - business logic
	if !slices.Contains(validStatuses, status) {
		return 0, fmt.Errorf("invalid status: %s", status)
	}

	_, err := s.db.Exec(updateJob, status, id)
	if err != nil {
		return 0, fmt.Errorf("update status failed: %w", err)
	}

	return id, nil
}

// Time spans -----

// Write span with null endtime and return span ID.
// startTime lets callers backdate a clock-in (e.g. forgot to clock in
// yesterday); pass time.Now() for the normal case.
func (s *SqlConn) WriteSpan(jobId int, startTime time.Time) (int, error) {
	res, err := s.db.Exec(writeSpan, jobId, startTime.UTC().Format(sqlTimeFormat))
	if err != nil {
		return 0, fmt.Errorf("write span failed: %w", err)
	}

	// Read job back in and return
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	return int(id), nil
}

// Update span with endtime. endTime lets callers close a span with a past
// time (e.g. clocking out for a span left open overnight).
func (s *SqlConn) UpdateSpan(spanId int, endTime time.Time) error {
	_, err := s.db.Exec(updateSpan, endTime.UTC().Format(sqlTimeFormat), spanId)
	if err != nil {
		return fmt.Errorf("update span failed: %w", err)
	}

	return nil
}
