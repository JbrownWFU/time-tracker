package main

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