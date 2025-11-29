package main

import (
	"html/template"
	"log"
	"net/http"
)

var tmpl = template.Must(template.ParseFiles("templates/index.gohtml"))

type PageData struct {
	Title   string
	Heading string
	Message string
	Items   []string
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title:   "Tiny Go Site",
		Heading: "Welcome to my tiny Go site",
		Message: "This page is rendered by Go using html/template.",
		Items: []string{
			"Go basics",
			"HTTP & JSON APIs",
			"Templates & HTML rendering",
		},
	}

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		log.Println("template error:", err)
		return
	}
}

func main() {
	http.HandleFunc("/", homeHandler)

	log.Println("Tiny HTML site on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
