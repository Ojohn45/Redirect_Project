package main

import (
	"bytes"
	"html/template"
	"log"
	"net/http"
	"sort"
)

var templates = template.Must(template.ParseGlob("templates/*.html"))

// One struct that every page can use. Each page only fills what it needs.
type PageData struct {
	User      *User // nil = guest
	Languages []Language
	Lang      Language
	Recent    []HistoryEntry

	// Admin pages
	HeadAdmin   *User
	UserRows    []UserRow
	TotalUsers  int
	OnlineUsers int
	Info        string // green message
	InviteLink  string // link to share by hand if the email could not be sent
	Token       string // invite token (accept page)
	InviteEmail string
	Existing    bool // the invited email already has an account
	CanAccept   bool // ...and the visitor is logged into it

	Error     string
	Created   bool // "account created" message on the login page
	NeedLogin bool // show the "please sign up" popup on page load

	// Keeps what the user typed when a form comes back with an error
	FullName string
	Username string
	Email    string
}

// render builds the page in memory first, so a template error
// never sends half a page to the browser.
func render(w http.ResponseWriter, name string, data PageData) {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, name, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

// Languages in a stable order (Go maps have no order).
func sortedLanguages() []Language {
	list := make([]Language, 0, len(languages))
	for _, l := range languages {
		list = append(list, l)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Slug < list[j].Slug })
	return list
}

// ---------- Public pages ----------

func landingPageHandler(w http.ResponseWriter, r *http.Request) {
	render(w, "landing.html", PageData{})
}

// Anyone can SEE the list of languages. Guests get the popup when they click one.
func languagesHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	render(w, "languages.html", PageData{
		User:      user,
		Languages: sortedLanguages(),
		NeedLogin: user == nil && r.URL.Query().Get("login_required") == "1",
	})
}

// ---------- Logged-in pages ----------

// guestRedirect sends a guest back to the languages page with the popup turned on.
func guestRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/languages?login_required=1", http.StatusSeeOther)
}

// The page people land on after logging in: a welcome + the Browse button.
func homePageHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	render(w, "home.html", PageData{User: user, Recent: recentHistory(user.ID, 5)})
}

// Shows the 10 sites for one language.
func languageHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		guestRedirect(w, r)
		return
	}

	lang, ok := languages[r.PathValue("slug")]
	if !ok {
		http.NotFound(w, r)
		return
	}
	render(w, "language.html", PageData{User: user, Lang: lang})
}

// Saves the visit in the user's history, then sends them to the site.
func redirectHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		guestRedirect(w, r)
		return
	}

	slug := r.PathValue("slug")
	resourceID := r.PathValue("resourceID")

	lang, ok := languages[slug]
	if !ok {
		http.NotFound(w, r)
		return
	}

	for _, res := range lang.Resources {
		if res.ID == resourceID {
			if err := addHistory(user.ID, slug, res.ID, res.Name); err != nil {
				log.Println("Could not save history:", err) // don't block the redirect
			}
			log.Printf("%s -> %s (%s)", user.Username, res.Name, res.URL)
			http.Redirect(w, r, res.URL, http.StatusFound)
			return
		}
	}

	http.NotFound(w, r)
}

func main() {
	initDB()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", landingPageHandler) // exactly "/", anything else = 404
	mux.HandleFunc("GET /languages", languagesHandler)
	mux.HandleFunc("GET /home", homePageHandler)
	mux.HandleFunc("GET /language/{slug}", languageHandler)
	mux.HandleFunc("GET /go/{slug}/{resourceID}", redirectHandler)

	mux.HandleFunc("GET /signup", signupPageHandler)
	mux.HandleFunc("POST /signup", signupHandler)
	mux.HandleFunc("GET /login", loginPageHandler)
	mux.HandleFunc("POST /login", loginHandler)
	mux.HandleFunc("POST /logout", logoutHandler)
	mux.HandleFunc("GET /goodbye", goodbyePageHandler)

	mux.HandleFunc("GET /admin", adminPageHandler)
	mux.HandleFunc("GET /admin/invite", inviteAdminPageHandler)
	mux.HandleFunc("POST /admin/invite", inviteAdminHandler)
	mux.HandleFunc("GET /admin/users", usersPageHandler)
	mux.HandleFunc("GET /admin/accept", acceptAdminPageHandler)
	mux.HandleFunc("POST /admin/accept", acceptAdminHandler)

	fileServer := http.FileServer(http.Dir("static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fileServer))

	log.Println("Server starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
