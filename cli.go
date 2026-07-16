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
	Where  WhereCmd  `cmd:"" help:"Show which job is currently clocked in, and for how long."`
	Report ReportCmd `cmd:"" help:"Print time entries for a job. Optionally write output to file."`
	About  AboutCmd  `cmd:"" help:"Print application info."`
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
	Name  string `arg:"" help:"Job name."`
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
	// Job   string `arg:"" help:"Job name to clock out of."`
	Notes string `arg:"" optional:"" help:"Optional notes for this time span."`
}

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

// What job am I clocked into?
// And for how long?
type WhereCmd struct{}

func (c *WhereCmd) Run(db *SqlDB.SqlConn) error {
	name, d, err := openSpanStatus(db)
	if err != nil {
		return err
	}

	fmt.Printf("%s\t%s\n", name, formatDuration(d))
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

// Print about / version
func (c *AboutCmd) Run(db *SqlDB.SqlConn) error {
	fmt.Printf("Version: %s\nDB: %s\nURL: %s\n",
		db.GetVersion(),
		db.GetPath(),
		db.GetURL(),
	)

	return nil
}

// There be vibes below

type ReportCmd struct {
	Name string `arg:"" optional:"" help:"Job name. If omitted, prints totals for all jobs."`
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
	if c.Name == "" {
		return c.runAll(db)
	}
	return c.runOne(db)
}

// runOne prints the full span-by-span breakdown for a single job.
func (c *ReportCmd) runOne(db *SqlDB.SqlConn) error {
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

	rows, total, err := spanRows(spans)
	if err != nil {
		return err
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

// runAll prints one total line per job, reusing the same span-duration
// math as runOne instead of a separate SQL aggregate.
func (c *ReportCmd) runAll(db *SqlDB.SqlConn) error {
	jobs, err := db.ListJobs()
	if err != nil {
		return err
	}
	if len(jobs) == 0 {
		fmt.Println("No jobs found.")
		return nil
	}

	var rows []jobTotalRow
	for _, j := range jobs {
		spans, err := db.ListSpansByJob(j.ID)
		if err != nil {
			return err
		}
		if len(spans) == 0 {
			continue
		}

		_, total, err := spanRows(spans)
		if err != nil {
			return err
		}
		rows = append(rows, jobTotalRow{Name: j.Name, Total: formatDuration(total)})
	}

	if c.File == "" {
		for _, r := range rows {
			fmt.Printf("%s\t%s\n", r.Name, r.Total)
		}
		return nil
	}

	switch strings.ToLower(filepath.Ext(c.File)) {
	case ".csv":
		if err := SqlDB.WriteReportCSV(jobTotalsCSVRows(rows), c.File); err != nil {
			return err
		}
	case ".md":
		if err := SqlDB.WriteReport(formatJobTotalsMarkdown(rows), c.File); err != nil {
			return err
		}
	default:
		if err := SqlDB.WriteReport(formatJobTotalsTxt(rows), c.File); err != nil {
			return err
		}
	}

	fmt.Printf("Report written to %s\n", c.File)
	return nil
}

// spanRows converts spans into display rows and sums their durations. An
// open span (nil EndTime) counts toward the total as time-since-start and
// is shown as "(in progress)".
func spanRows(spans []SqlDB.Span) ([]reportRow, time.Duration, error) {
	rows := make([]reportRow, 0, len(spans))
	var total time.Duration
	for _, sp := range spans {
		start, err := time.Parse(SqlDB.SqlTimeFormat, sp.StartTime)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to parse start time: %w", err)
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
			return nil, 0, fmt.Errorf("failed to parse end time: %w", err)
		}
		d := end.Sub(start)
		total += d
		rows = append(rows, reportRow{
			Start:    start.Local().Format("2006-01-02 15:04"),
			End:      end.Local().Format("2006-01-02 15:04"),
			Duration: formatDuration(d),
		})
	}

	return rows, total, nil
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

// jobTotalRow is a single job's total, used by the all-jobs report
// summary across the stdout, txt, markdown, and CSV renderers.
type jobTotalRow struct {
	Name  string
	Total string
}

func formatJobTotalsTxt(rows []jobTotalRow) string {
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "%s\t%s\n", r.Name, r.Total)
	}
	return b.String()
}

func formatJobTotalsMarkdown(rows []jobTotalRow) string {
	var b strings.Builder
	b.WriteString("| Job | Total |\n")
	b.WriteString("| --- | --- |\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | %s |\n", r.Name, r.Total)
	}
	return b.String()
}

func jobTotalsCSVRows(rows []jobTotalRow) [][]string {
	out := make([][]string, 0, len(rows)+1)
	out = append(out, []string{"job", "total"})
	for _, r := range rows {
		out = append(out, []string{r.Name, r.Total})
	}
	return out
}
