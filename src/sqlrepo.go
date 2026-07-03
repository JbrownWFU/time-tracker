package main

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

// Sqlite implementation of Repository
type SQLiteRepo struct {
	db *sql.DB
}

// Connect to db and ensure schema is ready
func NewSQLiteRepo(path string) (*SQLiteRepo, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	repo := &SQLiteRepo{
		db: db,
	}

	// Make Jobs
	if err := repo.makeJobsTable(); err != nil {
		return nil, err
	}
	// Add make Entries

	return repo, nil
}

func (r *SQLiteRepo) makeJobsTable() error {
	_, err := r.db.Exec(
		`
		create table if not exists jobs (
			id integer primary key,
			name text unique not null,
			description text,
			status text not null check (status in ('todo', 'active', 'done')),
			created_at text default current_timestamp
		)
		`,
	)

	return err
}

func (r *SQLiteRepo) CreateJob(j Job) error {
	_, err := r.db.Exec(
		`INSERT INTO jobs (name, description, status) VALUES (?, ?, ?)`,
		j.Name, j.Description, j.Status,
	)
	return err
}

func (r *SQLiteRepo) GetJob(name string) (Job, error) {
	var j Job
	err := r.db.QueryRow(
		`SELECT id, name, description, status FROM jobs WHERE name = ?`, name,
	).Scan(&j.ID, &j.Name, &j.Description, &j.Status)
	return j, err
}
