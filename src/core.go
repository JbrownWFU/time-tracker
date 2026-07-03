package main

import (
	"fmt"
	// "os"
)

// Core structs
// Ids will need to be handled per interface
// Designing for sqlite sequences
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
	// Write job to storage and return job
	WriteJob(j Job) (Job, error)
	// Return job by id
	GetJob(id int) (Job, error)
	// TODO add update job status

	// Start time
	OpenEntry(id int) error
	// Get current open entry on a job given a name
	// GetOpenEntry(id int) (Entry, error)
	// Stop time with optional note
	CloseEntry(id int, note string) error
}

// Job is a struct

// Internal service API structure
type Service struct {
	repo Repository
}

// Create a job and write to repo
func (s *Service) CreateJob(name string, description string, status string) (Job, error) {
	j := Job{
		Name: name,
		Description: description,
		Status: status,
	}

	job, err := s.repo.WriteJob(j)
	if err != nil {
		return Job{}, err
	}
	
	return job, err
}

// Internal function to find job by ID
func (s *Service) resolveJob(id int) (Job, error) {
	job, err := s.repo.GetJob(id)
	if err != nil {
		return Job{}, err
	}
	
	return job, nil
}

// func (s *Service) CreateJob(j *Job) error {
// 	if err := s.repo.CreateJob()
// } 

func main() {
	// Map Repo testing
	// repo := NewMapRepo()


	fmt.Println("Done")
}
