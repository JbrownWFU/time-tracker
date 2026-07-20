package server

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	SqlDB "time-tracker/src/sqldb"
)

// Embeddings - embed static/ and templates/

// Print requests to stdout
func pprint(r *http.Request) {
	ts := time.Now().Format("03:04:05PM")
	fmt.Printf("%s: %s -> %s\n", ts, r.Method, r.URL.Path)
}

type Server struct {
	db   *SqlDB.SqlConn
	tmpl *template.Template
}

// JobRow is jobs-table.html's view model: a Job plus the aggregate
// columns (entry count, total time) the template shows per row.
type JobRow struct {
	SqlDB.Job
	EntryCount int
	Total      string
}

// SpanRow is spans-table.html's view model: a Span with everything
// pre-formatted, so the template stays plain field access.
type SpanRow struct {
	ID       int
	Start    string
	End      string
	Duration string
	Note     string
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	m := (d % time.Hour) / time.Minute
	return fmt.Sprintf("%dh %dm", h, m)
}

func (s *Server) ui(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/ui.html")
}

// Generate jobs table fragment
func (s *Server) jobsTable(w http.ResponseWriter, r *http.Request) {
	pprint(r)

	jobs, err := s.db.ListJobs()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rows := make([]JobRow, 0, len(jobs))
	for _, job := range jobs {
		spans, err := s.db.GetJobSpans(job.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var total time.Duration
		for _, sp := range spans {
			if sp.EndTime == nil {
				total += time.Since(sp.StartTime.UTC())
				continue
			}
			total += sp.EndTime.Sub(sp.StartTime)
		}

		rows = append(rows, JobRow{
			Job:        job,
			EntryCount: len(spans),
			Total:      formatDuration(total),
		})
	}

	if err := s.tmpl.ExecuteTemplate(w, "jobs-table.html", rows); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Create a new job
func (s *Server) createJob(w http.ResponseWriter, r *http.Request) {
	pprint(r)

	// Read form
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	name := r.PostFormValue("name")
	desc := r.PostFormValue("desc")
	status := r.PostFormValue("status")

	fmt.Printf("name:%s\ndesc:%s\nstatus:%s\n", name, desc, status)

	// Create job
	if _, err := s.db.WriteJob(name, desc, status); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	// Reuse jobs table func
	s.jobsTable(w, r)
}

func (s *Server) deleteJob(w http.ResponseWriter, r *http.Request) {
	pprint(r)

	// ID comes from url path
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.db.DeleteJob(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.jobsTable(w, r)
}

// Generate a job's spans fragment, rendered into its expand panel
func (s *Server) jobSpans(w http.ResponseWriter, r *http.Request) {
	pprint(r)

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	spans, err := s.db.GetJobSpans(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	notes, err := s.db.GetJobNotes(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	notesBySpan := make(map[int][]SqlDB.Note)
	for _, n := range notes {
		notesBySpan[n.EntryID] = append(notesBySpan[n.EntryID], n)
	}

	rows := make([]SpanRow, 0, len(spans))
	for _, sp := range spans {
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

		rows = append(rows, SpanRow{
			ID:       sp.ID,
			Start:    sp.StartTime.Local().Format("2006-01-02 15:04"),
			End:      end,
			Duration: formatDuration(dur),
			Note:     strings.Join(noteText, "; "),
		})
	}

	if err := s.tmpl.ExecuteTemplate(w, "spans-table.html", rows); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Start time on a job
func (s *Server) clockIn(w http.ResponseWriter, r *http.Request) {
	pprint(r)

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ts := time.Now()
	if _, err := s.db.WriteSpan(id, ts); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Would update spans table here
	fmt.Println("Yay")
}

// Start the server on the specified port
func Serve(port int, db *SqlDB.SqlConn) error {
	// Parse templates dir
	tmpl := template.Must(
		template.ParseGlob("templates/*.html"),
	)

	server := &Server{
		db:   db,
		tmpl: tmpl,
	}

	host := fmt.Sprintf("localhost:%d", port)
	fmt.Printf("Server started on http://%s/ui\n", host)

	// Serve page
	http.HandleFunc("/ui", server.ui)
	// HTMX endpoints
	// Render jobs table
	http.HandleFunc("GET /jobs/table", server.jobsTable)
	// Create a job
	http.HandleFunc("POST /jobs/create", server.createJob)
	// Delete a job
	http.HandleFunc("DELETE /jobs/{id}", server.deleteJob)
	// Start time on a job
	http.HandleFunc("POST /jobs/{id}", server.clockIn)

	// Render a job's spans (expand panel)
	http.HandleFunc("GET /jobs/{id}/spans", server.jobSpans)

	if err := http.ListenAndServe(host, nil); err != nil {
		return err
	}

	return nil
}
