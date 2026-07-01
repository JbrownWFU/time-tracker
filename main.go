package main

import (
	// "modernc.org/sqlite"
	"fmt"
)

// Core structs
type Job struct {
	ID int
	Name string
}

// Create interface
type JobWriter interface {
	Write(job Job) error
	GetJob(id int) (Job, error)
}

// Create debug 


func main() {
	fmt.Println("Hello, World")
}