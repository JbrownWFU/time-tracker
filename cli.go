package main

import (
	"fmt"
	"time"

	SqlDB "time-tracker/src"
)

type CLI struct {
	DB string `help:"Path to sqlite database file." default:"time.db" type:"path"`

	Create CreateCmd `cmd:"" help:"Create a new job."`
	Status StatusCmd `cmd:"" help:"Update a job's status."`
	Show   ShowCmd   `cmd:"" help:"Show full details of a job."`
	List   ListCmd   `cmd:"" help:"List all jobs."`
	In     InCmd     `cmd:"" help:"Clock in to a job."`
	Out    OutCmd    `cmd:"" help:"Clock out of a job."`
	Report ReportCmd `cmd:"" help:"Print time entries for a job."`
}

type CreateCmd struct {
	Name   string `arg:"" help:"Job name."`
	Desc   string `help:"Job description." default:""`
	Status string `help:"Job status - must be one of 'todo', 'active', 'done'." default:"todo"`
}

func (c *CreateCmd) Run(db *SqlDB.SqlConn) error {
	_, err := db.WriteJob(c.Name, c.Desc, c.Status)
	fmt.Printf("Job created:\nName: %s\nDescription: %s\nStatus: %s\n", c.Name, c.Desc, c.Status)

	return err
}

type StatusCmd struct {
	Name   string `arg:"" help:"Job name."`
	Status string `arg:"" enum:"todo,active,done" help:"New status (todo, active, done)."`
}

func (c *StatusCmd) Run(db *SqlDB.SqlConn) error {
	id, err := db.ResolveJob(c.Name)
	if err != nil {
		return err
	}
	_, err = db.UpdateJobStatus(id, c.Status)
	return err
}

type ShowCmd struct {
	Name string `arg:"" help:"Job name."`
}

func (c *ShowCmd) Run(db *SqlDB.SqlConn) error {
	id, err := db.ResolveJob(c.Name)
	if err != nil {
		return err
	}
	job, err := db.GetJob(id)
	if err != nil {
		return err
	}
	fmt.Printf("%+v\n", job)
	return nil
}

type ListCmd struct{}

func (c *ListCmd) Run(db *SqlDB.SqlConn) error {
	jobs, err := db.ListJobs()
	if err != nil {
		return err
	}
	if len(jobs) == 0 {
		fmt.Println("No jobs found.")
		return nil
	}
	fmt.Printf("ID: Name: Status: Desc:\n")
	for _, j := range jobs {
		fmt.Printf("%d\t%s\t%s\t%s\n", j.ID, j.Name, j.Status, j.Desc)
	}
	return nil
}

type InCmd struct {
	Job string `arg:"" help:"Job name to clock in to."`
}

func (c *InCmd) Run(db *SqlDB.SqlConn) error {
	id, err := db.ResolveJob(c.Job)
	if err != nil {
		return err
	}
	ts := time.Now()
	_, err = db.WriteSpan(id, ts)
	
	fmt.Printf("Started: %s\n", ts.Format("2006-01-02 15:04:05"))
	return err
}

type OutCmd struct {
	Job   string `arg:"" help:"Job name to clock out of."`
	Notes string `arg:"" optional:"" help:"Optional notes for this time span."`
}

func (c *OutCmd) Run(db *SqlDB.SqlConn) error {
	jobId, err := db.ResolveJob(c.Job)
	if err != nil {
		return err
	}
	spanId, err := db.GetOpenSpan()
	if err != nil {
		return err
	}
	if spanId == 0 {
		return fmt.Errorf("no open span to close")
	}
	span, err := db.GetSpan(spanId)
	if err != nil {
		return err
	}
	if span.JobID != jobId {
		return fmt.Errorf("open span belongs to a different job")
	}  

	ts := time.Now()
	fmt.Printf("Ended: %s\n", ts.Format("2006-01-02 15:04:05"))

	if err := db.UpdateSpan(spanId, time.Now()); err != nil {
		return err
	}
	if c.Notes != "" {
		_, err = db.WriteNote(spanId, c.Notes)
	}
	return err
}

type ReportCmd struct {
	Name string `arg:"" help:"Job name."`
}

func (c *ReportCmd) Run(db *SqlDB.SqlConn) error {
	id, err := db.ResolveJob(c.Name)
	if err != nil {
		return err
	}
	spans, err := db.ListSpansByJob(id)
	if err != nil {
		return err
	}
	if len(spans) == 0 {
		fmt.Printf("No time entries found for %q.\n", c.Name)
		return nil
	}

	fmt.Printf("Time entries for %q:\n", c.Name)
	var total time.Duration
	for _, sp := range spans {
		start, err := time.Parse(SqlDB.SqlTimeFormat, sp.StartTime)
		if err != nil {
			return fmt.Errorf("failed to parse start time: %w", err)
		}

		if sp.EndTime == nil {
			d := time.Since(start.UTC())
			total += d
			fmt.Printf("%s -> (in progress)\t%s\n", start.Local().Format("2006-01-02 15:04"), formatDuration(d))
			continue
		}

		end, err := time.Parse(SqlDB.SqlTimeFormat, *sp.EndTime)
		if err != nil {
			return fmt.Errorf("failed to parse end time: %w", err)
		}
		d := end.Sub(start)
		total += d
		fmt.Printf("%s -> %s\t%s\n", start.Local().Format("2006-01-02 15:04"), end.Local().Format("2006-01-02 15:04"), formatDuration(d))
	}
	fmt.Printf("Total: %s\n", formatDuration(total))

	return nil
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	m := (d % time.Hour) / time.Minute
	return fmt.Sprintf("%dh %dm", h, m)
}
