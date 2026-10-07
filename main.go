package main

import (
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

func landingPageHandler(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "landing.html", nil)
}

func loginPageHandler(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "login.html", nil)
}

func main() {
	initDB()
	defer db.Close()

	mux := http.NewServeMux()
	fileServer := http.FileServer(http.Dir("static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fileServer))

	mux.HandleFunc("GET /", landingPageHandler)
	mux.HandleFunc("GET /languages", homeHandler) // your existing language list — same handler as before, just a new path
	mux.HandleFunc("GET /language/{slug}", languageHandler)
	mux.HandleFunc("GET /go/{slug}/{resourceID}", redirectHandler)
	mux.HandleFunc("GET /signup", signupPageHandler)
	mux.HandleFunc("POST /signup", signupHandler)
	mux.HandleFunc("GET /login", loginPageHandler)
}
