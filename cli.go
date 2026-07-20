package main

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	Server "time-tracker/src/server"
	SqlDB "time-tracker/src/sqldb"
)

type CLI struct {
	DB string `help:"Path to sqlite database file. Defaults to ~/.tracker/time.db." type:"path"`

	About  AboutCmd  `cmd:"" help:"Print application info."`
	Create CreateCmd `cmd:"" help:"Create a new job."`
	Delete DeleteCmd `cmd:"" help:"Delete a job and all its time spans."`
	Edit   EditCmd   `cmd:"" help:"Edit a job's name, description, and/or status."`
	Status StatusCmd `cmd:"" help:"Shorthand for 'edit <job> --status <status>'."`
	Show   ShowCmd   `cmd:"" help:"Show full details of a job."`
	List   ListCmd   `cmd:"" help:"List all jobs."`
	In     InCmd     `cmd:"" help:"Clock in to a job."`
	Out    OutCmd    `cmd:"" help:"Clock out of a job."`
	Report ReportCmd `cmd:"" help:"Print all time entries for a job."`
	Where  WhereCmd  `cmd:"" help:"Show which job is currently clocked in, and for how long."`
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

	spans, err := db.GetJobSpans(id)
	if err != nil {
		return err
	}

	total, openSince := summarizeSpans(spans)

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
	Notes  string `arg:"" optional:"" help:"Optional notes for this time span."`
	Delete bool   `help:"Discard the open span instead of closing it (undo an accidental clock-in)." default:"false"`
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

	if c.Delete {
		if err := db.DeleteSpan(spanId); err != nil {
			return err
		}
		fmt.Printf("Discarded span %d.\n", spanId)
		return nil
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

	if err := Server.Serve(c.Port, db, assets); err != nil {
		return err
	}

	return nil
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

// TODO figure out --from --to
type ReportCmd struct {
	Name   string `arg:"Job to report on"`
	From   string `help:"Start Date (YYYY-MM-DD)" xor:"range"`
	To     string `help:"End Date (YYYY-MM-DD)"`
	Today  bool   `help:"Report on today" xor:"range"`
	Week   bool   `help:"Report on this week" xor:"range"`
	Format string `help:"Output format" enum:"text,csv" default:"text"`
}

// Report on job - full time span printing
// Will support output to txt, md (just a different extension), csv (just text comma seperated values), json (? would be cool)
func (c *ReportCmd) Run(db *SqlDB.SqlConn) error {
	id, err := db.ResolveJob(c.Name)
	if err != nil {
		return err
	}

	job, err := db.GetJob(id)
	if err != nil {
		return err
	}

	spans, err := db.GetJobSpans(id)
	if err != nil {
		return err
	}

	notes, err := db.GetJobNotes(id)
	if err != nil {
		return err
	}

	from, to, err := c.dateRange(time.Now())
	if err != nil {
		return err
	}
	spans = filterSpansByRange(spans, from, to)

	out, err := formatReport(c.Format, job, spans, notes)
	if err != nil {
		return err
	}

	fmt.Print(out)
	return nil
}

// dateRange resolves the command's flags into a [from, to) window relative
// to now. A nil bound means unbounded on that side; both nil means no
// filtering at all (the --from/--to/--today/--week default case).
func (c *ReportCmd) dateRange(now time.Time) (from, to *time.Time, err error) {
	switch {
	case c.Today:
		start := startOfDay(now)
		end := start.AddDate(0, 0, 1)
		return &start, &end, nil

	case c.Week:
		start := startOfWeek(now)
		end := start.AddDate(0, 0, 7)
		return &start, &end, nil

	case c.From != "":
		start, err := time.ParseInLocation("2006-01-02", c.From, time.Local)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid --from date: %w", err)
		}
		if c.To == "" {
			return &start, nil, nil
		}
		endDay, err := time.ParseInLocation("2006-01-02", c.To, time.Local)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid --to date: %w", err)
		}
		// --to is the last day included, so the exclusive bound is the day after.
		end := endDay.AddDate(0, 0, 1)
		return &start, &end, nil

	case c.To != "":
		return nil, nil, fmt.Errorf("--to requires --from")

	default:
		return nil, nil, nil
	}
}

// startOfDay returns midnight, same day and location, as t.
func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// startOfWeek returns midnight on the Monday of t's week.
func startOfWeek(t time.Time) time.Time {
	start := startOfDay(t)
	offset := (int(start.Weekday()) + 6) % 7 // Weekday(): Sunday=0..Saturday=6
	return start.AddDate(0, 0, -offset)
}

// filterSpansByRange keeps only spans that started within [from, to).
// A nil bound is unbounded on that side; from == to == nil returns spans unchanged.
func filterSpansByRange(spans []SqlDB.Span, from, to *time.Time) []SqlDB.Span {
	if from == nil && to == nil {
		return spans
	}

	var filtered []SqlDB.Span
	for _, sp := range spans {
		if from != nil && sp.StartTime.Before(*from) {
			continue
		}
		if to != nil && !sp.StartTime.Before(*to) {
			continue
		}
		filtered = append(filtered, sp)
	}
	return filtered
}

// formatReportText renders a job's spans as plain text: one line per span
// (start, end, duration) plus a total line. Signature (job, spans) -> (string, error)
// is the shape every future formatter should share, so a --format flag can
// later dispatch to formatReportJSON/CSV/MD the same way.
func formatReportText(job SqlDB.Job, spans []SqlDB.Span, notes []SqlDB.Note) (string, error) {
	var b strings.Builder

	notesBySpan := make(map[int][]SqlDB.Note)
	for _, n := range notes {
		notesBySpan[n.EntryID] = append(notesBySpan[n.EntryID], n)
	}

	fmt.Fprintf(&b, "Report: %s\n", job.Name)

	for _, sp := range spans {
		start := sp.StartTime.Local().Format("2006-01-02 15:04")

		end := "open"
		dur := time.Since(sp.StartTime)
		if sp.EndTime != nil {
			end = sp.EndTime.Local().Format("2006-01-02 15:04")
			dur = sp.EndTime.Sub(sp.StartTime)
		}

		fmt.Fprintf(&b, "[%d] %s -> %s\t%s\n", sp.ID, start, end, formatDuration(dur))
		for _, n := range notesBySpan[sp.ID] {
			fmt.Fprintf(&b, "  note: %s\n", n.Content)
		}
	}

	total, _ := summarizeSpans(spans)
	fmt.Fprintf(&b, "Total: %s\n", formatDuration(total))

	return b.String(), nil
}

func formatReportCSV(job SqlDB.Job, spans []SqlDB.Span, notes []SqlDB.Note) (string, error) {
	var b strings.Builder
	w := csv.NewWriter(&b)

	notesBySpan := make(map[int][]SqlDB.Note)
	for _, n := range notes {
		notesBySpan[n.EntryID] = append(notesBySpan[n.EntryID], n)
	}

	if err := w.Write([]string{"id", "job", "start", "end", "duration", "minutes", "notes"}); err != nil {
		return "", err
	}

	for _, sp := range spans {
		start := sp.StartTime.Local().Format("2006-01-02 15:04")

		end := "open"
		dur := time.Since(sp.StartTime)
		if sp.EndTime != nil {
			end = sp.EndTime.Local().Format("2006-01-02 15:04")
			dur = sp.EndTime.Sub(sp.StartTime)
		}

		var noteText []string
		for _, n := range notesBySpan[sp.ID] {
			noteText = append(noteText, n.Content)
		}

		row := []string{
			strconv.Itoa(sp.ID),
			job.Name,
			start,
			end,
			formatDuration(dur),
			strconv.FormatFloat(dur.Minutes(), 'f', 1, 64),
			strings.Join(noteText, "; "),
		}
		if err := w.Write(row); err != nil {
			return "", err
		}
	}

	w.Flush()
	return b.String(), w.Error()
}

func formatReport(format string, job SqlDB.Job, spans []SqlDB.Span, notes []SqlDB.Note) (string, error) {
	switch format {
	case "text":
		return formatReportText(job, spans, notes)
	case "csv":
		return formatReportCSV(job, spans, notes)
	default:
		return "", fmt.Errorf("unknown format %q", format)
	}
}

// Helpers

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

	return job.Name, time.Since(span.StartTime.UTC()), nil
}

// summarizeSpans reduces a job's spans down to the total tracked duration
// and, if one span is still open, the time it was clocked in since. Shared
// by ShowCmd today; a future Report command can fold over the same spans
// (e.g. bucket by day) without re-touching the DB or re-parsing times.
func summarizeSpans(spans []SqlDB.Span) (total time.Duration, openSince *time.Time) {
	for _, sp := range spans {
		if sp.EndTime == nil {
			start := sp.StartTime.UTC()
			openSince = &start
			total += time.Since(start)
			continue
		}
		total += sp.EndTime.Sub(sp.StartTime)
	}
	return total, openSince
}
