package main

import (
	"database/sql"
	"fmt"
	"time"
)

// Core testing
// Break out into structured project

// Core structs
// Job id wil be handled by Sql sequence to ensure uniquness across launches
type Job struct {
	ID          int
	Name        string
	Description string
	Status      string
}

type Entry struct {
	ID        int
	JobID     int
	StartTime string
	EndTime   sql.NullString
	Note      sql.NullString
}

// General Interface for data storage
type Repository interface {
	WriteJob(j Job) error
	WriteEntryStart(e Entry) error
	WriteEntryEnd(e Entry) error

	ReadJob(id int) (Job, error)
	ReadEntry(id int) (Entry, error)
}

// Implement interface
// Debug type - in memory
type TestRepo struct {
	// Store jobs
	jobStore   map[int]Job
	entryStore map[int]Entry
}

// Instantiate DB / internal storage
func NewTestRepo() *TestRepo {
	return &TestRepo{
		jobStore:   make(map[int]Job),
		entryStore: make(map[int]Entry),
	}
}

// Job management

// Write a job to storage
func (t *TestRepo) WriteJob(j Job) error {
	// Write job
	t.jobStore[j.ID] = j
	return nil
}

// Read a job by ID
func (t *TestRepo) ReadJob(id int) (Job, error) {
	job, ok := t.jobStore[id]

	if !ok {
		return Job{}, fmt.Errorf("No job with id %d found ", id)
	}

	return job, nil
}

// Entry management

// Clock in
func (t *TestRepo) WriteEntryStart(e Entry) error {
	t.entryStore[e.ID] = e
	return nil
}

// Clock out
func (t *TestRepo) WriteEntryEnd(e Entry) error {
	ts := time.Now().Format(time.RFC3339)

	entry, ok := t.entryStore[e.ID]
	if !ok {
		return fmt.Errorf("No entry with id %d found ", e.ID)
	}

	entry.EndTime.String = ts
	entry.EndTime.Valid = true

	t.entryStore[entry.ID] = entry

	return nil
}

func (t *TestRepo) ReadEntry(id int) (Entry, error) {
	entry, ok := t.entryStore[id]

	if !ok {
		return Entry{}, fmt.Errorf("No entry with id %d found ", id)
	}

	return entry, nil
}

// Test consumer
// In the real application this could be useful to store a path to the file
// Will make in .track config in $HOME but still could be useful for other meta data
type Service struct {
	repo      Repository
	createdAt string
}

func (s *Service) TestWriteJob(j Job) error {
	// Write
	if err := s.repo.WriteJob(j); err != nil {
		return err
	}

	t := time.Now().Format(time.RFC3339)
	fmt.Println("Job written @", t)
	return nil
}

func (s *Service) TestWriteEntryStart(e Entry) error {
	// Write
	if err := s.repo.WriteEntryStart(e); err != nil {
		return err
	}

	t := time.Now().Format(time.RFC3339)
	fmt.Printf("Entry written @ %s with job id: %d\n", t, e.JobID)
	return nil
}

func (s *Service) TestWriteEntryEnd(e Entry) error {
	// Write
	if err := s.repo.WriteEntryEnd(e); err != nil {
		return err
	}

	t := time.Now().Format(time.RFC3339)
	fmt.Printf("Entry written @ %s with job id: %d\n", t, e.JobID)
	return nil
}

func (s *Service) GetJobById(id int) (Job, error) {
	job, err := s.repo.ReadJob(id)
	if err != nil {
		return Job{}, err
	}

	return job, nil
}

func (s *Service) GetEntryById(id int) (Entry, error) {
	entry, err := s.repo.ReadEntry(id)
	if err != nil {
		return Entry{}, err
	}

	return entry, nil
}

func (s *Service) GetCreatedAt() string {
	return s.createdAt
}

func main() {
	job := Job{
		1,
		"myJob",
		"A test job",
		"todo",
	}

	// Instantiate instance of DB / map
	repo := NewTestRepo()

	t := time.Now().Format(time.RFC3339)

	// Create instance of Service with test repo
	service := Service{repo: repo, createdAt: t}
	fmt.Printf("Service created @ %s\n", service.GetCreatedAt())

	service.TestWriteJob(job)

	gotJob, err := service.GetJobById(job.ID)
	if err != nil {
		fmt.Println("Error in Service.GetJobById")
		fmt.Println(err)
	}

	fmt.Printf("Job with id %d:\n", job.ID)
	fmt.Println(gotJob)

	// Test write an entry

	entry := Entry{
		1,
		1,
		t,
		sql.NullString{String: "", Valid: false},
		sql.NullString{String: "", Valid: false},
	}

	// Clock in
	fmt.Println("In ->")
	service.TestWriteEntryStart(entry)
	
	// Do some work
	fmt.Println("Working...")
	time.Sleep(time.Second * 5)
	
	// Clock out
	fmt.Println("<- Out")
	service.TestWriteEntryEnd(entry)

	gotEntry, err := service.GetEntryById(entry.ID)
	if err != nil {
		fmt.Println("Error in Service.GetJobById")
		fmt.Println(err)
	}

	fmt.Printf("Entry with id %d:\n", gotEntry.ID)
	fmt.Println(gotEntry)

}
