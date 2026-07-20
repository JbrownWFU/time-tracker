package server

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	SqlDB "time-tracker/src/sqldb"
)

// Print requests to stdout
func pprint(r *http.Request) {
	ts := time.Now().Format("03:04:05PM")
	fmt.Printf("%s: %s -> %s\n", ts, r.Method, r.URL.Path)
}

// pathID reads the {id} path value shared by nearly every route below.
func pathID(r *http.Request) (int, error) {
	return strconv.Atoi(r.PathValue("id"))
}

type Server struct {
	db     *SqlDB.SqlConn
	tmpl   *template.Template
	assets embed.FS
}

// JobRow is job-display-row.html's view model: a Job plus the aggregate
// columns (entry count, total time) and clock-in state the template shows.
type JobRow struct {
	SqlDB.Job
	EntryCount int
	Total      string
	Open       bool // this job has the currently open span
	Locked     bool // a different job has the open span, so In is disabled
	OOB        bool // render with hx-swap-oob (out-of-band refresh triggered by a span edit/delete)
}

// SpanRow is span-display-row.html/span-edit-row.html's view model: a Span
// with everything pre-formatted, so the templates stay plain field access.
type SpanRow struct {
	ID         int
	Start      string // display: "2006-01-02 15:04"
	End        string // display: "2006-01-02 15:04", or "open"
	StartInput string // edit value: "2006-01-02T15:04" (datetime-local format)
	EndInput   string // edit value: "2006-01-02T15:04", empty if open
	Duration   string
	Note       string
}

// StatusView is status-bar.html's view model.
type StatusView struct {
	DBPath    string
	ClockedIn bool
	JobName   string
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	m := (d % time.Hour) / time.Minute
	return fmt.Sprintf("%dh %dm", h, m)
}

func (s *Server) ui(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, s.assets, "static/ui.html")
}

func (s *Server) buildStatusView() (StatusView, error) {
	openJobID, err := s.openJobID()
	if err != nil {
		return StatusView{}, err
	}

	view := StatusView{DBPath: s.db.GetPath()}
	if openJobID != 0 {
		job, err := s.db.GetJob(openJobID)
		if err != nil {
			return StatusView{}, err
		}
		view.ClockedIn = true
		view.JobName = job.Name
	}

	return view, nil
}

// Render the header status bar: DB path + clocked-in/out state
func (s *Server) statusBar(w http.ResponseWriter, r *http.Request) {
	pprint(r)

	view, err := s.buildStatusView()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.tmpl.ExecuteTemplate(w, "status-bar.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// renderStatusBarOOB writes an out-of-band status-bar.html fragment,
// appended after In/Out's main response so the clocked-in pill updates
// live instead of needing a poll. status-bar.html has no wrapping element
// of its own (it's swapped into #status-bar's innerHTML on page load), so
// the OOB wrapper here supplies the "swap into #status-bar" targeting
// that a normal render doesn't need. Best-effort: headers are already
// committed by the caller's main response, so an error here can only be
// logged, not turned into a 500.
func (s *Server) renderStatusBarOOB(w http.ResponseWriter) {
	view, err := s.buildStatusView()
	if err != nil {
		fmt.Printf("renderStatusBarOOB: %v\n", err)
		return
	}

	fmt.Fprint(w, `<div hx-swap-oob="innerHTML:#status-bar">`)
	if err := s.tmpl.ExecuteTemplate(w, "status-bar.html", view); err != nil {
		fmt.Printf("renderStatusBarOOB: %v\n", err)
	}
	fmt.Fprint(w, `</div>`)
}

// openJobID returns the ID of the job with the currently open span, or 0
// if nothing is clocked in.
func (s *Server) openJobID() (int, error) {
	spanId, err := s.db.GetOpenSpan()
	if err != nil {
		return 0, err
	}
	if spanId == 0 {
		return 0, nil
	}

	span, err := s.db.GetSpan(spanId)
	if err != nil {
		return 0, err
	}

	return span.JobID, nil
}

// buildJobRow computes a job's aggregate columns and clock-in state,
// given the ID of whichever job (if any) currently has the open span.
func (s *Server) buildJobRow(job SqlDB.Job, openJobID int) (JobRow, error) {
	spans, err := s.db.GetJobSpans(job.ID)
	if err != nil {
		return JobRow{}, err
	}

	var total time.Duration
	for _, sp := range spans {
		if sp.EndTime == nil {
			total += time.Since(sp.StartTime.UTC())
			continue
		}
		total += sp.EndTime.Sub(sp.StartTime)
	}

	return JobRow{
		Job:        job,
		EntryCount: len(spans),
		Total:      formatDuration(total),
		Open:       job.ID == openJobID,
		Locked:     openJobID != 0 && job.ID != openJobID,
	}, nil
}

// Generate jobs table fragment
func (s *Server) jobsTable(w http.ResponseWriter, r *http.Request) {
	pprint(r)

	jobs, err := s.db.ListJobs()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	openJobID, err := s.openJobID()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rows := make([]JobRow, 0, len(jobs))
	for _, job := range jobs {
		row, err := s.buildJobRow(job, openJobID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		rows = append(rows, row)
	}

	if err := s.tmpl.ExecuteTemplate(w, "jobs-table.html", rows); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// renderJobRow re-renders a single job's display row (job-display-row.html),
// the target for In/Out/Edit/Save/Cancel so only that job's row changes -
// its spans panel, open or closed, is untouched.
func (s *Server) renderJobRow(w http.ResponseWriter, id int) {
	row, err := s.buildJobRowByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.tmpl.ExecuteTemplate(w, "job-display-row.html", row); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// renderJobRowOOB writes an out-of-band job-display-row.html fragment,
// appended after a span edit/delete's main response so that job's entry
// count/total refresh even though the span row was the actual request
// target. Best-effort: headers are already committed by the caller's main
// response, so an error here can only be logged, not turned into a 500.
func (s *Server) renderJobRowOOB(w http.ResponseWriter, jobId int) {
	row, err := s.buildJobRowByID(jobId)
	if err != nil {
		fmt.Printf("renderJobRowOOB: %v\n", err)
		return
	}
	row.OOB = true

	if err := s.tmpl.ExecuteTemplate(w, "job-display-row.html", row); err != nil {
		fmt.Printf("renderJobRowOOB: %v\n", err)
	}
}

// renderOtherJobRowsOOB OOB-refreshes every job row except excludeID after
// a clock in/out changes global open-job state - every other row's Locked
// field (whether its In button is disabled) depends on that same state,
// not just the row for the job that was actually clicked.
func (s *Server) renderOtherJobRowsOOB(w http.ResponseWriter, excludeID int) {
	jobs, err := s.db.ListJobs()
	if err != nil {
		fmt.Printf("renderOtherJobRowsOOB: %v\n", err)
		return
	}

	openJobID, err := s.openJobID()
	if err != nil {
		fmt.Printf("renderOtherJobRowsOOB: %v\n", err)
		return
	}

	for _, job := range jobs {
		if job.ID == excludeID {
			continue
		}

		row, err := s.buildJobRow(job, openJobID)
		if err != nil {
			fmt.Printf("renderOtherJobRowsOOB: %v\n", err)
			continue
		}
		row.OOB = true

		if err := s.tmpl.ExecuteTemplate(w, "job-display-row.html", row); err != nil {
			fmt.Printf("renderOtherJobRowsOOB: %v\n", err)
		}
	}
}

func (s *Server) buildJobRowByID(id int) (JobRow, error) {
	job, err := s.db.GetJob(id)
	if err != nil {
		return JobRow{}, err
	}

	openJobID, err := s.openJobID()
	if err != nil {
		return JobRow{}, err
	}

	return s.buildJobRow(job, openJobID)
}

// Create a new job
func (s *Server) createJob(w http.ResponseWriter, r *http.Request) {
	pprint(r)

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	name := r.PostFormValue("name")
	desc := r.PostFormValue("desc")
	status := r.PostFormValue("status")

	if _, err := s.db.WriteJob(name, desc, status); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Reuse jobs table func so the new row appears without a full page reload
	s.jobsTable(w, r)
}

// Display a single job's row - the Cancel target after Edit
func (s *Server) jobRow(w http.ResponseWriter, r *http.Request) {
	pprint(r)

	id, err := pathID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.renderJobRow(w, id)
}

// Edit-mode row for a job's name/status/desc
func (s *Server) editJobRow(w http.ResponseWriter, r *http.Request) {
	pprint(r)

	id, err := pathID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	row, err := s.buildJobRowByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.tmpl.ExecuteTemplate(w, "job-edit-row.html", row); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Save a job's edited name/status/desc
func (s *Server) updateJob(w http.ResponseWriter, r *http.Request) {
	pprint(r)

	id, err := pathID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	name := r.PostFormValue("name")
	desc := r.PostFormValue("desc")
	status := r.PostFormValue("status")

	if err := s.db.UpdateJobDetails(id, &name, &desc, &status); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.renderJobRow(w, id)
}

func (s *Server) deleteJob(w http.ResponseWriter, r *http.Request) {
	pprint(r)

	id, err := pathID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.db.DeleteJob(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Empty response: hx-target="closest tbody" hx-swap="outerHTML" removes
	// just this job's tbody, no need to re-render the rest of the table.
}

// buildSpanRow formats one span (plus its notes) for display or edit.
func (s *Server) buildSpanRow(id int) (SpanRow, error) {
	sp, err := s.db.GetSpan(id)
	if err != nil {
		return SpanRow{}, err
	}

	notes, err := s.db.GetSpanNotes(id)
	if err != nil {
		return SpanRow{}, err
	}
	var noteText []string
	for _, n := range notes {
		noteText = append(noteText, n.Content)
	}

	end := "open"
	endInput := ""
	dur := time.Since(sp.StartTime)
	if sp.EndTime != nil {
		end = sp.EndTime.Local().Format("2006-01-02 15:04")
		endInput = sp.EndTime.Local().Format("2006-01-02T15:04")
		dur = sp.EndTime.Sub(sp.StartTime)
	}

	return SpanRow{
		ID:         sp.ID,
		Start:      sp.StartTime.Local().Format("2006-01-02 15:04"),
		End:        end,
		StartInput: sp.StartTime.Local().Format("2006-01-02T15:04"),
		EndInput:   endInput,
		Duration:   formatDuration(dur),
		Note:       strings.Join(noteText, "; "),
	}, nil
}

// renderSpanRow re-renders a single span's display row - the target for
// Delete's OOB-adjacent main response and the Cancel-after-Edit fragment.
func (s *Server) renderSpanRow(w http.ResponseWriter, id int) {
	row, err := s.buildSpanRow(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.tmpl.ExecuteTemplate(w, "span-display-row.html", row); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Generate a job's spans fragment, rendered into its expand panel
func (s *Server) jobSpans(w http.ResponseWriter, r *http.Request) {
	pprint(r)

	id, err := pathID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	spans, err := s.db.GetJobSpans(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rows := make([]SpanRow, 0, len(spans))
	for _, sp := range spans {
		row, err := s.buildSpanRow(sp.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		rows = append(rows, row)
	}

	if err := s.tmpl.ExecuteTemplate(w, "spans-table.html", rows); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Display a single span's row - the Cancel target after Edit
func (s *Server) spanRow(w http.ResponseWriter, r *http.Request) {
	pprint(r)

	id, err := pathID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.renderSpanRow(w, id)
}

// Edit-mode row for a span's start/end time and note
func (s *Server) editSpanRow(w http.ResponseWriter, r *http.Request) {
	pprint(r)

	id, err := pathID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	row, err := s.buildSpanRow(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.tmpl.ExecuteTemplate(w, "span-edit-row.html", row); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

const spanInputLayout = "2006-01-02T15:04"

// Save a span's edited start/end time and/or note, then OOB-refresh its
// job's row so the entry count/total don't go stale.
func (s *Server) updateSpan(w http.ResponseWriter, r *http.Request) {
	pprint(r)

	id, err := pathID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	span, err := s.db.GetSpan(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var start, end *time.Time
	if v := r.PostFormValue("start"); v != "" {
		t, err := time.ParseInLocation(spanInputLayout, v, time.Local)
		if err != nil {
			http.Error(w, "invalid start time", http.StatusBadRequest)
			return
		}
		start = &t
	}
	if v := r.PostFormValue("end"); v != "" {
		t, err := time.ParseInLocation(spanInputLayout, v, time.Local)
		if err != nil {
			http.Error(w, "invalid end time", http.StatusBadRequest)
			return
		}
		end = &t
	}

	if start != nil || end != nil {
		if err := s.db.UpdateSpanDetails(id, start, end); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if err := s.db.SetSpanNote(id, r.PostFormValue("note")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.renderSpanRow(w, id)
	s.renderJobRowOOB(w, span.JobID)
}

// Delete a span, then OOB-refresh its job's row so the entry count/total
// don't go stale.
func (s *Server) deleteSpan(w http.ResponseWriter, r *http.Request) {
	pprint(r)

	id, err := pathID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	span, err := s.db.GetSpan(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.db.DeleteSpan(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Empty main response: hx-target="closest tr" hx-swap="outerHTML" removes
	// this span's row; the OOB fragment below refreshes the job row alongside it.
	s.renderJobRowOOB(w, span.JobID)
}

// Start time on a job
func (s *Server) clockIn(w http.ResponseWriter, r *http.Request) {
	pprint(r)

	id, err := pathID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if _, err := s.db.WriteSpan(id, time.Now()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.renderJobRow(w, id)
	s.renderOtherJobRowsOOB(w, id)
	s.renderStatusBarOOB(w)
}

// Clock out of a job, with optional notes on the closed span
func (s *Server) clockOut(w http.ResponseWriter, r *http.Request) {
	pprint(r)

	id, err := pathID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	spanId, err := s.db.GetOpenSpan()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if spanId == 0 {
		http.Error(w, "no open span to close", http.StatusBadRequest)
		return
	}

	if err := s.db.UpdateSpan(spanId, time.Now()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if notes := r.PostFormValue("notes"); notes != "" {
		if _, err := s.db.WriteNote(spanId, notes); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	s.renderJobRow(w, id)
	s.renderOtherJobRowsOOB(w, id)
	s.renderStatusBarOOB(w)
}

// Start the server on the specified port. assets is the embedded static/
// and templates/ trees (see the repo root's assets.go), so the built
// binary needs neither directory alongside it at runtime.
func Serve(port int, db *SqlDB.SqlConn, assets embed.FS) error {
	tmpl := template.Must(
		template.ParseFS(assets, "templates/*.html"),
	)

	staticFS, err := fs.Sub(assets, "static")
	if err != nil {
		return fmt.Errorf("failed to load embedded static assets: %w", err)
	}

	server := &Server{
		db:     db,
		tmpl:   tmpl,
		assets: assets,
	}

	host := fmt.Sprintf("localhost:%d", port)
	fmt.Printf("Server started on http://%s/ui\n", host)

	// Serve page
	http.HandleFunc("/ui", server.ui)
	// Serve static assets (css/js) referenced by ui.html
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServerFS(staticFS)))
	// HTMX endpoints
	// Render jobs table
	http.HandleFunc("GET /jobs/table", server.jobsTable)
	// Render header status bar (DB path + clocked-in state)
	http.HandleFunc("GET /status", server.statusBar)
	// Create a job
	http.HandleFunc("POST /jobs/create", server.createJob)
	// Display a single job's row (Cancel target after Edit)
	http.HandleFunc("GET /jobs/{id}", server.jobRow)
	// Edit-mode row for a job
	http.HandleFunc("GET /jobs/{id}/edit", server.editJobRow)
	// Save a job's edited details
	http.HandleFunc("PUT /jobs/{id}", server.updateJob)
	// Delete a job
	http.HandleFunc("DELETE /jobs/{id}", server.deleteJob)
	// Start time on a job
	http.HandleFunc("POST /jobs/{id}", server.clockIn)
	// Clock out of a job
	http.HandleFunc("POST /jobs/{id}/out", server.clockOut)

	// Render a job's spans (expand panel)
	http.HandleFunc("GET /jobs/{id}/spans", server.jobSpans)
	// Display a single span's row (Cancel target after Edit)
	http.HandleFunc("GET /spans/{id}", server.spanRow)
	// Edit-mode row for a span
	http.HandleFunc("GET /spans/{id}/edit", server.editSpanRow)
	// Save a span's edited details
	http.HandleFunc("PUT /spans/{id}", server.updateSpan)
	// Delete a span
	http.HandleFunc("DELETE /spans/{id}", server.deleteSpan)

	if err := http.ListenAndServe(host, nil); err != nil {
		return err
	}

	return nil
}
