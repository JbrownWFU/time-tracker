package main

import (
	"fmt"
	"maps"
)

type MapRepo struct {
	jobs map[int]Job
}

func NewMapRepo() *MapRepo {
	jobs := make(map[int]string)

	return &MapRepo{
		jobs: jobs
	}
}

func (r *MapRepo) WriteJob(j job) error {

} 