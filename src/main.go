package main

import (
	"fmt"
	"log"
	"time"
)

/*
TODO
- Put notes in their own table they dont live on Spans
- Update sql funcs to just work with IDs
- Function to take in time from user for backdating spans
*/

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
	// _, err = Conn.WriteJob("birdJob2", "a job for birds", "todo")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	
	// Get existing job id
	jobId, err := Conn.ResolveJob("birdJob4")
	if err != nil {
		log.Fatal(err)
	}
	
	// Update job
	/*
	_, err = Conn.UpdateJobStatus(jobId, "active")
	if err != nil {
		log.Fatal(err)
	}
	*/

	// Test clock in
	// TODO need way to resolve job name -> id
	startId, err := Conn.WriteSpan(jobId, time.Now())
	if err != nil {
		log.Fatal(err)
	}
	
	fmt.Printf("Start Time: %s\n", time.Now())
	fmt.Println("Working...")
	
	// Check for open span
	openId, err := Conn.GetOpenSpan()
	if err != nil {
		log.Fatal(err)
	}
	
	fmt.Printf("Open span id: %d\n", openId)
	
	// // Do work
	time.Sleep(time.Second * 3)
	
	if err := Conn.UpdateSpan(startId, time.Now()); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("End Time: %s\n", time.Now())
	
	// Check for open span when there is none
	openId, err = Conn.GetOpenSpan()
	if err != nil {
		log.Fatal(err)
	}
	
	fmt.Printf("Open span id: %d\n", openId)
}
