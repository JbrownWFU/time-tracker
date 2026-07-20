package main

import (
	"fmt"
	"time"

	Server "time-tracker/src/server"
	SqlDB "time-tracker/src/sqldb"
)

type CLI struct {
	DB string `help:"Path to sqlite database file. Defaults to ~/.tracker/time.db." type:"path"`

	Create CreateCmd `cmd:"" help:"Create a new job."`
	Edit   EditCmd   `cmd:"" help:"Edit a job's name, description, and/or status."`
	Status StatusCmd `cmd:"" help:"Shorthand for 'edit <job> --status <status>'."`
	Delete DeleteCmd `cmd:"" help:"Delete a job and all its time spans."`
	Show   ShowCmd   `cmd:"" help:"Show full details of a job."`
	List   ListCmd   `cmd:"" help:"List all jobs."`
	In     InCmd     `cmd:"" help:"Clock in to a job."`
	Out    OutCmd    `cmd:"" help:"Clock out of a job."`
	Where  WhereCmd  `cmd:"" help:"Show which job is currently clocked in, and for how long."`
	About  AboutCmd  `cmd:"" help:"Print application info."`
	Serve  ServeCmd  `cmd:"" help:"Run with localhost web interface."`
}

// Create a new job
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

// Edit job details
type EditCmd struct {
	Name      string  `arg:"" help:"Job name."`
	NewName   *string `name:"name" help:"New job name."`
	NewDesc   *string `name:"desc" help:"New job description."`
	NewStatus *string `name:"status" enum:"todo,active,done," help:"New status (todo, active, done)."`
}

func (c *EditCmd) Run(db *SqlDB.SqlConn) error {
	if c.NewName == nil && c.NewDesc == nil && c.NewStatus == nil {
		return fmt.Errorf("nothing to edit: pass at least one of --name, --desc, --status")
	}
	id, err := db.ResolveJob(c.Name)
	if err != nil {
		return err
	}
	return db.UpdateJobDetails(id, c.NewName, c.NewDesc, c.NewStatus)
}

type StatusCmd struct {
	Name   string `arg:"" help:"Job name."`
	Status string `arg:"" enum:"todo,active,done" help:"New status (todo, active, done)."`
}

// Update a jobs status
func (c *StatusCmd) Run(db *SqlDB.SqlConn) error {
	id, err := db.ResolveJob(c.Name)
	if err != nil {
		return err
	}
	return db.UpdateJobDetails(id, nil, nil, &c.Status)
}

type DeleteCmd struct {
	Name  string `arg:"" help:"Job name."`
	Force bool   `help:"Skip the confirmation prompt." default:"false"`
}

// Delete a job and its entries
func (c *DeleteCmd) Run(db *SqlDB.SqlConn) error {
	if !c.Force {
		fmt.Printf("Delete job %q and all its time spans? [y/N] ", c.Name)
		var resp string
		fmt.Scanln(&resp)
		if resp != "y" && resp != "Y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	id, err := db.ResolveJob(c.Name)
	if err != nil {
		return err
	}
	if err := db.DeleteJob(id); err != nil {
		return err
	}

	fmt.Printf("Deleted job %q.\n", c.Name)
	return nil
}

type ShowCmd struct {
	Name string `arg:"" help:"Job name."`
}

// Show job details
func (c *ShowCmd) Run(db *SqlDB.SqlConn) error {
	id, err := db.ResolveJob(c.Name)
	if err != nil {
		return err
	}
	job, err := db.GetJob(id)
	if err != nil {
		return err
	}
	spans, err := db.ListSpansByJob(id)
	if err != nil {
		return err
	}

	var total time.Duration
	var openSince *time.Time
	for _, sp := range spans {
		start, err := time.Parse(SqlDB.SqlTimeFormat, sp.StartTime)
		if err != nil {
			return fmt.Errorf("failed to parse start time: %w", err)
		}

		if sp.EndTime == nil {
			start := start.UTC()
			openSince = &start
			total += time.Since(start)
			continue
		}

		end, err := time.Parse(SqlDB.SqlTimeFormat, *sp.EndTime)
		if err != nil {
			return fmt.Errorf("failed to parse end time: %w", err)
		}
		total += end.Sub(start)
	}

	fmt.Printf("Name:         %s\n", job.Name)
	fmt.Printf("Status:       %s\n", job.Status)
	if job.Desc != "" {
		fmt.Printf("Description:  %s\n", job.Desc)
	}
	fmt.Printf("Time entries: %d\n", len(spans))
	fmt.Printf("Total time:   %s\n", formatDuration(total))
	if openSince != nil {
		fmt.Printf("Clocked in:   since %s (%s elapsed)\n",
			openSince.Local().Format("2006-01-02 15:04"), formatDuration(time.Since(*openSince)))
	} else {
		fmt.Printf("Clocked in:   no\n")
	}

	return nil
}

type ListCmd struct{}

// List all jobs
func (c *ListCmd) Run(db *SqlDB.SqlConn) error {
	jobs, err := db.ListJobs()
	if err != nil {
		return err
	}
	if len(jobs) == 0 {
		fmt.Println("No jobs found.")
		return nil
	}
	fmt.Printf("Name: Status: Desc:\n")
	for _, j := range jobs {
		fmt.Printf("%s\t%s\t%s\n", j.Name, j.Status, j.Desc)
	}
	return nil
}

type InCmd struct {
	Job string `arg:"" help:"Job name to clock in to."`
}

// Clock into a job
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
	// Job   string `arg:"" help:"Job name to clock out of."`
	Notes string `arg:"" optional:"" help:"Optional notes for this time span."`
}

// Clock out of a job
func (c *OutCmd) Run(db *SqlDB.SqlConn) error {
	spanId, err := db.GetOpenSpan()
	if err != nil {
		return err
	}
	if spanId == 0 {
		return fmt.Errorf("no open span to close")
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

// What job am I clocked into now and for how long?
type WhereCmd struct{}

// Show currently clocked in job and duration if any
func (c *WhereCmd) Run(db *SqlDB.SqlConn) error {
	name, d, err := openSpanStatus(db)
	if err != nil {
		return err
	}

	fmt.Printf("%s\t%s\n", name, formatDuration(d))
	return nil
}

type ServeCmd struct {
	Port int `arg:"" default:"8283" help:"Port to serve on (default: 8283)"`
}

// Run the application as a local web ui with optional port (default 8283)
func (c *ServeCmd) Run(db *SqlDB.SqlConn) error {
	// Start server

	if err := Server.Serve(c.Port, db); err != nil {
		return err
	}

	return nil
}

// openSpanStatus returns the job name and elapsed duration for the
// currently open span, if any.
func openSpanStatus(db *SqlDB.SqlConn) (string, time.Duration, error) {
	spanId, err := db.GetOpenSpan()
	if err != nil {
		return "", 0, err
	}
	if spanId == 0 {
		return "", 0, fmt.Errorf("not clocked in")
	}

	span, err := db.GetSpan(spanId)
	if err != nil {
		return "", 0, err
	}
	job, err := db.GetJob(span.JobID)
	if err != nil {
		return "", 0, err
	}

	start, err := time.Parse(SqlDB.SqlTimeFormat, span.StartTime)
	if err != nil {
		return "", 0, fmt.Errorf("failed to parse start time: %w", err)
	}

	return job.Name, time.Since(start.UTC()), nil
}

type AboutCmd struct{}

func (c *AboutCmd) Run(db *SqlDB.SqlConn) error {
	fmt.Printf("Version: %s (%s)\n", version, commit)
	fmt.Printf("Built:   %s\n", date)
	fmt.Printf("Repo:    %s\n", githubURL)
	fmt.Printf("DB:      %s\n", db.GetPath())
	return nil
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	m := (d % time.Hour) / time.Minute
	return fmt.Sprintf("%dh %dm", h, m)
}
