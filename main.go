package main

import (
	"fmt"
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

// Make job

func main() {
	job := Job{
		1,
		"myJob",
		"A test job",
		"todo",
	}

	fmt.Printf("Job with id %d:\n", job.ID)
	fmt.Println(job)
}
