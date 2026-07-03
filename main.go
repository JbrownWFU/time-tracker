package main

import (
	"fmt"
	"time"
)

// Core structs
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
	EndTime   string
	Note      string
}

// General Interface for data storage
type Repository interface {
	WriteJob(j Job) error
	WriteEntry(e Entry) error

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
		jobStore: make(map[int]Job),
		entryStore: make(map[int]Entry),
	}
}

// Job management

func (t *TestRepo) WriteJob(j Job) error {
	// Write job
	t.jobStore[j.ID] = j
	return nil
}

func (t *TestRepo) ReadJob(id int) (Job, error) {
	job, ok := t.jobStore[id]

	if !ok {
		return Job{}, fmt.Errorf("No job with id %d found ", id)
	}

	return job, nil
}

// Entry management

func (t *TestRepo) WriteEntry(e Entry) error {
	// Write job
	t.entryStore[e.ID] = e
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

func (s *Service) TestWrite(j Job) error {
	// Write
	if err := s.repo.WriteJob(j); err != nil {
		return err
	}

	t := time.Now().Format(time.RFC3339)
	fmt.Println("Job written @ ", t)
	return nil
}

func (s *Service) GetJobById(id int) (Job, error) {
	job, err := s.repo.ReadJob(id)
	if err != nil {
		return Job{}, err
	}

	return job, nil
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
	storage := Service{repo: repo, createdAt: t}
	fmt.Printf("Service created @ %s\n", storage.GetCreatedAt())

	storage.TestWrite(job)

	gotJob, err := storage.GetJobById(job.ID)
	if err != nil {
		fmt.Println("Error in Service.GetJobById")
		fmt.Println(err)
	}

	fmt.Printf("Job with id %d:\n", job.ID)
	fmt.Println(gotJob)
}
