package main

import (
	"fmt"
	"log"
)

// Core structs
type Job struct {
	ID          int
	Name        string
	Desc 		string
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
	// Test sql
	Conn, err := NewSqlConn("dev.db")
	if err != nil {
		log.Fatal(err)
	}
	defer Conn.Close()
	
	if err = Conn.MakeTables(); err != nil {
		log.Fatal(err)
	}
	
	job, err := Conn.WriteJob("turtleJob", "a job for turtles", "todo")
	if err != nil {
		log.Fatal(err)
	}
	
	fmt.Println(job)
}
