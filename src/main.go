package main

import (
	"fmt"
	"log"
)

// TODO
// Put notes in their own table they dont live on Spans

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
	
	job, err := Conn.WriteJob("birdJob4", "a job for birds", "todo")
	if err != nil {
		log.Fatal(err)
	}
	
	_, err = Conn.GetJob(1)
	if err != nil {
		log.Fatal(err)
	}

	// fmt.Println(job)
	
	// Test clock in
	// TODO need way to resolve job name -> id
	start, err := Conn.WriteSpan(job.ID, nil)
	if err != nil {
		log.Fatal(err)
	}
	
	fmt.Println("Start")
	fmt.Println(start)
	
	// Update job
	// Works
	// updatedJob, err := Conn.UpdateJobStatus(job, "active")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	
	// fmt.Println(updatedJob)
}
