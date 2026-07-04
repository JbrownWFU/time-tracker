package main

import (
	"fmt"
	"os"
	"slices"
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
	// Write job to storage and return job with unique ID
	WriteJob(j Job) (Job, error)
	// Return job by id
	// Might remove if not needed
	GetJob(id int) (Job, error)
	// Return job by name
	GetJobByName(name string) (Job, error)
	// TODO add update job status
	UpdateJob(id int, status string) (Job, error)

	// Start time
	// OpenEntry(id int) error
	// // Get current open entry on a job given a name
	// // GetOpenEntry(id int) (Entry, error)
	// // Stop time with optional note
	// CloseEntry(id int, note string) error
}

// Internal service API structure
type Service struct {
	repo Repository
}

var validStatuses = []string{
	"todo",
	"active",
	"done",
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

func (s *Service) UpdateJob(name string, status string) (Job, error) {
	// Check status is valid
	if !slices.Contains(validStatuses, status) {
		return Job{}, fmt.Errorf("Invalid status %s: must be one of %q\n", status, &validStatuses)
	}

	// Get job
	job, err := s.repo.GetJobByName(name)
	if err != nil {
		return Job{}, err
	}
	
	// Update
	updatedJob, err := s.repo.UpdateJob(job.ID, status)
	if err != nil {
		return Job{}, err
	}

	return updatedJob, nil
}

func main() {
	// Map Repo testing
	repo := NewTestRepo()
	
	service := Service{repo: repo}
	fmt.Println("Service started")
	
	job, err := service.CreateJob(
		"Bird Job",
		"A robot bird project",
		"todo",
	)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	
	fmt.Printf("Job: %+v\n", job)
	
	// Update job
	updatedJob, err := service.UpdateJob(job.Name, "active")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Printf("Job: %+v\n", updatedJob)

	fmt.Println("Done")
}
