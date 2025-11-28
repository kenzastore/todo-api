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

type Todo struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
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

	// parse frontend template (for /)
	tmpl = template.Must(template.ParseFiles("static/index.html"))

	// API routes
	http.HandleFunc("/todos", todosHandler)   // GET, POST
	http.HandleFunc("/todos/", todoItemHandler) // PUT, DELETE

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

func todosHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getTodosHandler(w, r)
	case http.MethodPost:
		createTodoHandler(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func todoItemHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPut:
		updateTodoHandler(w, r)
	case http.MethodDelete:
		deleteTodoHandler(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func getTodosHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`SELECT id, title, done FROM todos ORDER BY id`)
	if err != nil {
		log.Println("getTodos query:", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var todos []Todo
	for rows.Next() {
		var t Todo
		if err := rows.Scan(&t.ID, &t.Title, &t.Done); err != nil {
			log.Println("getTodos scan:", err)
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		todos = append(todos, t)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(todos)
}

func createTodoHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title string `json:"title"`
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

	res, err := db.Exec(`INSERT INTO todos (title, done) VALUES (?, 0)`, title)
	if err != nil {
		log.Println("createTodo insert:", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	id64, _ := res.LastInsertId()

	todo := Todo{
		ID:    int(id64),
		Title: title,
		Done:  false,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(todo)
}

func updateTodoHandler(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/todos/")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var body struct {
		Title string `json:"title"`
		Done  bool   `json:"done"`
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
		`UPDATE todos SET title = ?, done = ? WHERE id = ?`,
		title, body.Done, id,
	)
	if err != nil {
		log.Println("updateTodo update:", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		http.Error(w, "todo not found", http.StatusNotFound)
		return
	}

	todo := Todo{
		ID:    id,
		Title: title,
		Done:  body.Done,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(todo)
}

func deleteTodoHandler(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/todos/")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	res, err := db.Exec(`DELETE FROM todos WHERE id = ?`, id)
	if err != nil {
		log.Println("deleteTodo delete:", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		http.Error(w, "todo not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
