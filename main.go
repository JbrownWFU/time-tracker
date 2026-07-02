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
	GetJob(id int) (Job, error)
	// Add write entry
	// Add read entry
}

// Implement interface
// Debug type - in memory
type TestRepo struct {
	// Store jobs
	store map[int]Job
}

// Instantiate DB / internal storage
func NewTestRepo() *TestRepo {
	return &TestRepo{
		store: make(map[int]Job),
	}
}

// Satisfies Repository 
func (t *TestRepo) WriteJob(j Job) error {
	// Write job 
	t.store[j.ID] = j
	return nil
}

// Satisfies Repository 
func (t *TestRepo) GetJob(id int) (Job, error) {
	job, ok := t.store[id]
	
	if !ok {
		return Job{}, fmt.Errorf("No job with id %d found ", id)
	}
	
	return job, nil
}

// Test consumer
// In the real application this could be useful to store a path to the file
// Will make in .track config in $HOME but still could be useful for other meta data
type Storage struct {
	repo Repository
	createdAt string
}

func (s *Storage) TestWrite(j Job) error {
	// Write
	if err := s.repo.WriteJob(j); err != nil {
		return err
	}
	
	t := time.Now().Format(time.RFC3339)
	fmt.Println("Job written @ ", t)
	return nil
}

func (s *Storage) GetJobById(id int) (Job, error) {
	job, err := s.repo.GetJob(id)
	if err != nil {
		return Job{}, err
	}
	
	return job, nil
}

// Operate on interface directly
func writeJobRepo(r Repository, j Job) error {
	if err := r.WriteJob(j); err != nil {
		return err
	}
	
	return nil
}

func getJobRepo(r Repository, id int) (Job, error) {
	if job, err := r.GetJob(id); err != nil {
		return Job{}, err
	} else {
		return job, nil
	}
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

	// Create instance of Storage with test repo
	storage := Storage{repo: repo, createdAt: t}
	storage.TestWrite(job)
	
	gotJob, err := storage.GetJobById(job.ID)
	if err != nil {
		fmt.Println("Error in Storage.GetJobById")
		fmt.Println(err)
	}
	
	fmt.Printf("Job with id %d:\n", job.ID)
	fmt.Println(gotJob)
	
	// Test write on interface directly
	fmt.Println("By Repo:")
	
	if err = writeJobRepo(repo, job); err != nil {
		fmt.Println("Error writing job")
		fmt.Println(err)
		return
	}
	
	fmt.Println("Job writen via repo")
	
	job, err = getJobRepo(repo, job.ID)
	if err != nil {
		fmt.Println("Error getting job")
		fmt.Println(err)
		return
	}
	
	fmt.Println("Job from repo")
	fmt.Println(job)
}
