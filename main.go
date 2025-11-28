package main

import (
	"database/sql"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

type Note struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

var (
	db   *sql.DB
	tmpl *template.Template
)

func main() {
	// DB config - for now hardcoded; later you can move to env vars
	dsn := os.Getenv("TODO_DB_DSN")
	if dsn == "" {
		// user:password@tcp(host:port)/dbname
		dsn = "cloud:cloud.kenzastore.my.id@tcp(127.0.0.1:3306)/cloud?parseTime=true&charset=utf8mb4&loc=Local"
	}

	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("sql.Open:", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatal("db.Ping:", err)
	}
	log.Println("Connected to MariaDB")

	// Create table if not exists
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS notes (
			id INT AUTO_INCREMENT PRIMARY KEY,
			title TEXT NOT NULL,
			content TEXT
		)
	`)
	if err != nil {
		log.Fatal("create table:", err)
	}

	// parse frontend template (for /)
	tmpl = template.Must(template.ParseFiles("static/index.html"))

	// API routes
	http.HandleFunc("/notes", notesHandler)   // GET, POST
	http.HandleFunc("/notes/", noteItemHandler) // PUT, DELETE

	// Frontend
	http.HandleFunc("/", frontHandler)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Println("Server running at http://localhost:" + port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// --------- Handlers ----------

func frontHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if err := tmpl.Execute(w, nil); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		log.Println("template error:", err)
	}
}

func notesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getNotesHandler(w, r)
	case http.MethodPost:
		createNoteHandler(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func noteItemHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPut:
		updateNoteHandler(w, r)
	case http.MethodDelete:
		deleteNoteHandler(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func getNotesHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`SELECT id, title, content FROM notes ORDER BY id DESC`)
	if err != nil {
		log.Println("getNotes query:", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.Title, &n.Content); err != nil {
			log.Println("getNotes scan:", err)
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		notes = append(notes, n)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(notes)
}

func createNoteHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	title := strings.TrimSpace(body.Title)
	if title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}

	res, err := db.Exec(`INSERT INTO notes (title, content) VALUES (?, ?)`, title, body.Content)
	if err != nil {
		log.Println("createNote insert:", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	id64, _ := res.LastInsertId()

	note := Note{
		ID:      int(id64),
		Title:   title,
		Content: body.Content,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(note)
}

func updateNoteHandler(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/notes/")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var body struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	title := strings.TrimSpace(body.Title)
	if title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}

	res, err := db.Exec(
		`UPDATE notes SET title = ?, content = ? WHERE id = ?`,
		title, body.Content, id,
	)
	if err != nil {
		log.Println("updateNote update:", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		http.Error(w, "note not found", http.StatusNotFound)
		return
	}

	note := Note{
		ID:      id,
		Title:   title,
		Content: body.Content,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(note)
}

func deleteNoteHandler(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/notes/")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	res, err := db.Exec(`DELETE FROM notes WHERE id = ?`, id)
	if err != nil {
		log.Println("deleteNote delete:", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		http.Error(w, "note not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
