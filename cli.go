package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	SqlDB "time-tracker/src"
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
	Report ReportCmd `cmd:"" help:"Print time entries for a job. Optionally write output to file."`
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

func (c *StatusCmd) Run(db *SqlDB.SqlConn) error {
	id, err := db.ResolveJob(c.Name)
	if err != nil {
		return err
	}
	return db.UpdateJobDetails(id, nil, nil, &c.Status)
}

type DeleteCmd struct {
	Name    string `arg:"" help:"Job name."`
	Force bool   `help:"Skip the confirmation prompt." default:"false"`
}

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

	if err := db.DeleteJob(c.Name); err != nil {
		return err
	}

	fmt.Printf("Deleted job %q.\n", c.Name)
	return nil
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
	File string `name:"file" short:"o" help:"Write report to a file instead of stdout. Format is inferred from the extension (.csv, .md, else plain text)."`
}

// reportRow is a single formatted time entry, shared across the stdout,
// txt, markdown, and CSV renderers.
type reportRow struct {
	Start    string
	End      string
	Duration string
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

	rows := make([]reportRow, 0, len(spans))
	var total time.Duration
	for _, sp := range spans {
		start, err := time.Parse(SqlDB.SqlTimeFormat, sp.StartTime)
		if err != nil {
			return fmt.Errorf("failed to parse start time: %w", err)
		}

		if sp.EndTime == nil {
			d := time.Since(start.UTC())
			total += d
			rows = append(rows, reportRow{
				Start:    start.Local().Format("2006-01-02 15:04"),
				End:      "(in progress)",
				Duration: formatDuration(d),
			})
			continue
		}

		end, err := time.Parse(SqlDB.SqlTimeFormat, *sp.EndTime)
		if err != nil {
			return fmt.Errorf("failed to parse end time: %w", err)
		}
		d := end.Sub(start)
		total += d
		rows = append(rows, reportRow{
			Start:    start.Local().Format("2006-01-02 15:04"),
			End:      end.Local().Format("2006-01-02 15:04"),
			Duration: formatDuration(d),
		})
	}

	if c.File == "" {
		fmt.Printf("Time entries for %q:\n", c.Name)
		for _, r := range rows {
			fmt.Printf("%s -> %s\t%s\n", r.Start, r.End, r.Duration)
		}
		fmt.Printf("Total: %s\n", formatDuration(total))
		return nil
	}

	switch strings.ToLower(filepath.Ext(c.File)) {
	case ".csv":
		if err := SqlDB.WriteReportCSV(csvRows(rows), c.File); err != nil {
			return err
		}
	case ".md":
		if err := SqlDB.WriteReport(formatMarkdown(rows), c.File); err != nil {
			return err
		}
	default:
		if err := SqlDB.WriteReport(formatTxt(rows), c.File); err != nil {
			return err
		}
	}

	fmt.Printf("Report written to %s\n", c.File)
	return nil
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	m := (d % time.Hour) / time.Minute
	return fmt.Sprintf("%dh %dm", h, m)
}

func formatTxt(rows []reportRow) string {
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "%s -> %s\t%s\n", r.Start, r.End, r.Duration)
	}
	return b.String()
}

func formatMarkdown(rows []reportRow) string {
	var b strings.Builder
	b.WriteString("| Start | End | Duration |\n")
	b.WriteString("| --- | --- | --- |\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", r.Start, r.End, r.Duration)
	}
	return b.String()
}

func csvRows(rows []reportRow) [][]string {
	out := make([][]string, 0, len(rows)+1)
	out = append(out, []string{"start", "end", "duration"})
	for _, r := range rows {
		out = append(out, []string{r.Start, r.End, r.Duration})
	}
	return out
}
