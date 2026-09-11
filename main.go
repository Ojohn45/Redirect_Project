package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
)

var templates = template.Must(template.ParseGlob("templates/*.html"))

func homeHandler(w http.ResponseWriter, r *http.Request) {
	err := templates.ExecuteTemplate(w, "home.html", languages)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Println(err)
	}
}

func languageHandler(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug") // Go 1.22+ path parameter

	lang, ok := languages[slug]
	if !ok {
		http.NotFound(w, r)
		return
	}

	err := templates.ExecuteTemplate(w, "language.html", lang)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Println(err)
	}
}

func redirectHandler(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	resourceID := r.PathValue("resourceID")

	lang, ok := languages[slug]
	if !ok {
		http.NotFound(w, r)
		return
	}

	for _, res := range lang.Resources {
		if res.ID == resourceID {
			log.Printf("Redirecting to %s (%s)", res.Name, res.URL)
			http.Redirect(w, r, res.URL, http.StatusFound)
			return
		}
	}

	http.NotFound(w, r)
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", homeHandler)
	mux.HandleFunc("GET /language/{slug}", languageHandler)
	mux.HandleFunc("GET /go/{slug}/{resourceID}", redirectHandler)

	fmt.Println("Server starting on http://localhost:8080")
	http.ListenAndServe(":8080", mux)
}
