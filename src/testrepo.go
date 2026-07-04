package main

import (
	"fmt"
)

type TestRepo struct {
	jobs map[int]Job
}

func NewTestRepo() *TestRepo {
	jobs := make(map[int]Job)

	return &TestRepo{
		jobs: jobs,
	}
}

// Simulate global counter
func (r *TestRepo) WriteJob(j Job) (Job, error) {
	currId := len(r.jobs)
	j.ID = currId + 1
	r.jobs[j.ID] = j

	return j, nil
}

func (r *TestRepo) GetJob(id int) (Job, error) {
	job, ok := r.jobs[id]
	if !ok {
		return Job{}, fmt.Errorf("get: job not found with ID: %d", id)
	}

	return job, nil
}

func (r *TestRepo) GetJobByName(name string) (Job, error) {
	for _, job := range r.jobs {
		if job.Name == name {
			return job, nil
		}
	}

	return Job{}, fmt.Errorf("get: job not found with name: %s", name)
}

func (r *TestRepo) UpdateJob(id int, status string) (Job, error) {
	job, ok := r.jobs[id]
	if !ok {
		return Job{}, fmt.Errorf("update: job not found with ID: %d", id)
	}
	
	job.Status = status
	r.jobs[id] = job

	return job, nil
}
