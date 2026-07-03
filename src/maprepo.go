package main

import (
	"fmt"
)

type MapRepo struct {
	jobs map[int]Job
}

func NewMapRepo() *MapRepo {
	jobs := make(map[int]Job)

	return &MapRepo{
		jobs: jobs,
	}
}

func (r *MapRepo) WriteJob(j Job) error {
	fmt.Println("ok")
	return nil
} 