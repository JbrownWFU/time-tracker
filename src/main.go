package main

import (
	"fmt"
	"log"
	"time"
)

// TODO
// Put notes in their own table they dont live on Spans
// Update sql funcs to just work with IDs
// Function to take in time from user for backdating spans

// Core structs
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
	ID 		int
	EntryID int
	Content string
}

// Make job

func main() {
	// Test sql
	Conn, err := NewSqlConn("dev.db")
	if err != nil {
		log.Fatal(err)
	}
	defer Conn.Close()
	
	if err = Conn.MakeTables(); err != nil {
		log.Fatal(err)
	}
	
	// Create a new job
	jobId, err := Conn.WriteJob("birdJob", "a job for birds", "todo")
	if err != nil {
		log.Fatal(err)
	}

	// Update job
	_, err = Conn.UpdateJobStatus(jobId, "active")
	if err != nil {
		log.Fatal(err)
	}

	// Test clock in
	// TODO need way to resolve job name -> id
	startId, err := Conn.WriteSpan(jobId, time.Now())
	if err != nil {
		log.Fatal(err)
	}
	
	fmt.Printf("Start ID: %d\n", startId)
	fmt.Println("Working...")
	
	// Do work
	time.Sleep(time.Second * 5)
	
	if err := Conn.UpdateSpan(startId, time.Now()); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("end ID: %d\n", startId)
	
}
