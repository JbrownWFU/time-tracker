package SqlDB

import (
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
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

	// Job management

	writeJob = `
	insert into Jobs (name, desc, status, created_at)
	values (?, ?, ?, ?)
	`
	getJob = `
	select id, name, desc, status from Jobs
	where id = ?
	`
	updateJobDetails = `
	update Jobs set
		name = coalesce(?, name),
		desc = coalesce(?, desc),
		status = coalesce(?, status)
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

	// If we delete a job should we also delete spans?
	// Add option in cmd
	deleteJob = `
	delete from Jobs
	where id = ?
	`

	// Span management

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
	// Get currently clocked in job by name
	getOpenJobName = `
	select name from jobs
	where id = (select job_id from spans where end_time is null);
	`
	updateSpan = `
	update Spans set end_time = ?
	where id = ?
	`
	// Need to get job ID, so need to know job name
	deleteSpanByJob = `
	delete from Spans
	where job_id = ?
	`

	// Note management

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
	db   *sql.DB
	path string
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

	return SqlConn{db: db, path: path}, nil
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

// Delete Job and  delete spans associated with it.
// Will implement an archival feature to 'retire' jobs from the active list
// At that point whats the use in having the 'done' status?
func (s *SqlConn) DeleteJob(name string) error {
	// Get job ID
	id, err := s.ResolveJob(name)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete Spans first so we never leave spans orphaned by a deleted job
	if _, err = tx.Exec(deleteSpanByJob, id); err != nil {
		return err
	}

	// Delete Job
	if _, err = tx.Exec(deleteJob, id); err != nil {
		return err
	}

	return tx.Commit()
}

// Update a job's name, description, and/or status. A nil pointer leaves that
// field unchanged. Returns a friendly error if name collides with another job.
func (s *SqlConn) UpdateJobDetails(id int, name, desc, status *string) error {
	if status != nil && !slices.Contains(validStatuses, *status) {
		return fmt.Errorf("invalid status: %s", *status)
	}

	_, err := s.db.Exec(updateJobDetails,
		nullableString(name), nullableString(desc), nullableString(status), id)
	if err != nil {
		if name != nil && isUniqueConstraintErr(err) {
			return fmt.Errorf("a job named %q already exists", *name)
		}
		return fmt.Errorf("update job failed: %w", err)
	}

	return nil
}

func nullableString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func isUniqueConstraintErr(err error) bool {
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// Time spans -----

// Write span with null endtime and return span ID.
// startTime lets callers backdate a clock-in (e.g. forgot to clock in
// this morning); pass time.Now() for the normal case.
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

// Get open job name

func (s *SqlConn) GetPath() string {
	return s.path
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

// Reporting -----

// Write content to fileName
func WriteReport(content string, fileName string) error {
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)
	if err != nil {
		return err
	}

	return nil
}

// Write rows to fileName as CSV. The first row is expected to be the header.
func WriteReportCSV(rows [][]string, fileName string) error {
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	w := csv.NewWriter(file)
	if err := w.WriteAll(rows); err != nil {
		return err
	}

	return nil
}
