package SqlDB

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	_ "modernc.org/sqlite"
)

const (
	SqlTimeFormat = "2006-01-02 15:04:05"

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
	listJobs = `
	select id, name, desc, status from Jobs
	order by id
	`

	listSpansByJob = `
	select id, job_id, start_time, end_time from Spans
	where job_id = ?
	order by start_time
	`

	writeSpan = `
	insert into Spans (job_id, start_time)
	values (?, ?)
	`
	getSpan = `
	select * from Spans
	where id = ?
	`
	// Find open time entries
	getOpenSpanID = `
	select max(id) from Spans
	where end_time is null
	`
	updateSpan = `
	update Spans set end_time = ?
	where id = ?
	`

	writeNote = `
	insert into Notes (entry_id, content)
	values (?, ?)
	`
)

type Job struct {
	ID     int
	Name   string
	Desc   string
	Status string
}

type Span struct {
	ID        int
	JobID     int
	StartTime string
	EndTime   *string
}

type Note struct {
	ID      int
	EntryID int
	Content string
}

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

	if _, err = db.Exec(makeTables); err != nil {
		db.Close()
		return SqlConn{}, fmt.Errorf("make tables: %w", err)
	}

	return SqlConn{db: db}, nil
}

// Close DB Connection
func (s *SqlConn) Close() error {
	return s.db.Close()
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

// List all jobs
func (s *SqlConn) ListJobs() ([]Job, error) {
	rows, err := s.db.Query(listJobs)
	if err != nil {
		return nil, fmt.Errorf("list jobs failed: %w", err)
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.Name, &j.Desc, &j.Status); err != nil {
			return nil, fmt.Errorf("failed to scan job row: %w", err)
		}
		jobs = append(jobs, j)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate job rows: %w", err)
	}

	return jobs, nil
}

// List all spans for a job, ordered by start time
func (s *SqlConn) ListSpansByJob(jobId int) ([]Span, error) {
	rows, err := s.db.Query(listSpansByJob, jobId)
	if err != nil {
		return nil, fmt.Errorf("list spans failed: %w", err)
	}
	defer rows.Close()

	var spans []Span
	for rows.Next() {
		var sp Span
		if err := rows.Scan(&sp.ID, &sp.JobID, &sp.StartTime, &sp.EndTime); err != nil {
			return nil, fmt.Errorf("failed to scan span row: %w", err)
		}
		spans = append(spans, sp)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate span rows: %w", err)
	}

	return spans, nil
}

// Write job to storage
func (s *SqlConn) WriteJob(name string, desc string, status string) (int, error) {
	if !slices.Contains(validStatuses, status) {
		return 0, fmt.Errorf("invalid status: %s", status)
	}

	res, err := s.db.Exec(writeJob, name, desc, status, time.Now().UTC().Format(SqlTimeFormat))
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
	openId, err := s.GetOpenSpan()
	if err != nil {
		return 0, fmt.Errorf("failed to check for open span: %w", err)
	}
	if openId != 0 {
		openSpan, err := s.GetSpan(openId)
		if err != nil {
			return 0, fmt.Errorf("failed to look up open span: %w", err)
		}
		openJob, err := s.GetJob(openSpan.JobID)
		if err != nil {
			return 0, fmt.Errorf("failed to look up job for open span: %w", err)
		}
		return 0, fmt.Errorf("already clocked in to %q; clock out before starting a new one", openJob.Name)
	}

	res, err := s.db.Exec(writeSpan, jobId, startTime.UTC().Format(SqlTimeFormat))
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
	_, err := s.db.Exec(updateSpan, endTime.UTC().Format(SqlTimeFormat), spanId)
	if err != nil {
		return fmt.Errorf("update span failed: %w", err)
	}

	return nil
}

// open span testing
// Finds open span on table and returns ID
// Returns 0 if there are none
func (s *SqlConn) GetOpenSpan() (int, error) {
	var id sql.NullInt64
	err := s.db.QueryRow(getOpenSpanID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("could not get open span: %w", err)
	}

	if !id.Valid {
		return 0, nil
	}

	return int(id.Int64), nil
}

// Get span by ID
func (s *SqlConn) GetSpan(id int) (Span, error) {
	var sp Span
	err := s.db.QueryRow(getSpan, id).Scan(&sp.ID, &sp.JobID, &sp.StartTime, &sp.EndTime)
	if err != nil {
		return Span{}, err
	}

	return sp, nil
}

// Notes -----

// Write note attached to a span
func (s *SqlConn) WriteNote(spanId int, content string) (int, error) {
	res, err := s.db.Exec(writeNote, spanId, content)
	if err != nil {
		return 0, fmt.Errorf("write note failed: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	return int(id), nil
}
