package server

import (
	"fmt"
	"html/template"
	"net/http"
	"time"
	"strconv"

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

	if err := s.tmpl.ExecuteTemplate(w, "jobs-table.html", jobs); err != nil {
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

	if err := http.ListenAndServe(host, nil); err != nil {
		return err
	}

	return nil
}
