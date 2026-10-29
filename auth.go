package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"mime"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// ---------- Password rules ----------

func isValidPassword(password string) bool {
	hasLetter := false
	hasNumber := false

	for _, char := range password {
		switch {
		case unicode.IsLetter(char):
			hasLetter = true
		case unicode.IsDigit(char):
			hasNumber = true
		}
	}

	return hasLetter && hasNumber
}

// ---------- Sessions (the "you are logged in" cookie) ----------

const (
	sessionCookie = "session"
	sessionTTL    = 7 * 24 * time.Hour
)

type session struct {
	userID  int
	expires time.Time
}

var (
	sessions   = map[string]session{}
	sessionsMu sync.Mutex
)

func startSession(w http.ResponseWriter, userID int) error {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	token := hex.EncodeToString(b)

	sessionsMu.Lock()
	sessions[token] = session{userID: userID, expires: time.Now().Add(sessionTTL)}
	sessionsMu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// Secure: true, // <- turn this on once the site runs on https://
	})
	return nil
}

func endSession(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		sessionsMu.Lock()
		delete(sessions, c.Value)
		sessionsMu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
}

// currentUser returns the logged-in user, or nil for guests.
func currentUser(r *http.Request) *User {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return nil
	}

	sessionsMu.Lock()
	s, ok := sessions[c.Value]
	if ok && time.Now().After(s.expires) {
		delete(sessions, c.Value)
		ok = false
	}
	sessionsMu.Unlock()
	if !ok {
		return nil
	}

	u, found := findUserByID(s.userID)
	if !found {
		return nil
	}
	return &u
}

// ---------- Signup ----------

func signupPageHandler(w http.ResponseWriter, r *http.Request) {
	if currentUser(r) != nil {
		http.Redirect(w, r, "/home", http.StatusSeeOther)
		return
	}
	render(w, "signup.html", PageData{})
}

func signupHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	fullName := strings.TrimSpace(r.FormValue("full_name"))
	username := strings.TrimSpace(r.FormValue("username"))
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")

	// If something is wrong we show the form again with the error
	// and keep what the person already typed (except the password).
	fail := func(msg string) {
		w.WriteHeader(http.StatusBadRequest)
		render(w, "signup.html", PageData{Error: msg, FullName: fullName, Username: username, Email: email})
	}

	if fullName == "" || username == "" || email == "" || password == "" {
		fail("All fields are required")
		return
	}

	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		fail("Please enter a valid email address")
		return
	}

	if !isValidPassword(password) {
		fail("Password must contain both letters and numbers")
		return
	}
	if len(password) > 72 { // bcrypt only reads the first 72 bytes
		fail("Password is too long (max 72 characters)")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Println("Error hashing password:", err)
		return
	}

	_, err = createUser(fullName, username, email, string(hashedPassword))
	switch {
	case errors.Is(err, ErrUsernameTaken):
		fail("That username is already taken")
		return
	case errors.Is(err, ErrEmailTaken):
		fail("That email is already registered")
		return
	case err != nil:
		log.Println("Error saving user:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// "go" = run in the background, so the user doesn't wait for the email.
	go sendWelcomeEmail(email, fullName)

	http.Redirect(w, r, "/login?created=1", http.StatusSeeOther)
}

// ---------- Login / Logout ----------

// A fake hash so a wrong username takes as long as a wrong password.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("not-a-real-password1"), bcrypt.DefaultCost)

func loginPageHandler(w http.ResponseWriter, r *http.Request) {
	if currentUser(r) != nil {
		http.Redirect(w, r, "/home", http.StatusSeeOther)
		return
	}
	render(w, "login.html", PageData{Created: r.URL.Query().Get("created") == "1"})
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")

	user, found := findUserByUsername(username)
	hash := dummyHash
	if found {
		hash = []byte(user.PasswordHash)
	}
	err := bcrypt.CompareHashAndPassword(hash, []byte(password))

	if !found || err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		render(w, "login.html", PageData{Error: "Invalid username or password", Username: username})
		return
	}

	if err := startSession(w, user.ID); err != nil {
		log.Println("Error starting session:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/home", http.StatusSeeOther)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	endSession(w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ---------- Welcome email ----------
//
// Set these environment variables before starting the server:
//   SMTP_USER  your Gmail address
//   SMTP_PASS  a Gmail "App Password" (NOT your normal password)
// Optional: SMTP_HOST (default smtp.gmail.com), SMTP_PORT (default 587), SMTP_FROM

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func sendWelcomeEmail(toEmail, fullName string) {
	user := os.Getenv("SMTP_USER")
	pass := strings.ReplaceAll(os.Getenv("SMTP_PASS"), " ", "") // Google shows App Passwords with spaces
	if user == "" || pass == "" {
		log.Println("Email not configured (SMTP_USER / SMTP_PASS missing) - skipping welcome email to", toEmail)
		return
	}

	host := getenv("SMTP_HOST", "smtp.gmail.com")
	port := getenv("SMTP_PORT", "587")
	from := getenv("SMTP_FROM", user)

	firstName := strings.Fields(fullName)
	name := fullName
	if len(firstName) > 0 {
		name = firstName[0]
	}

	body := "Hi " + name + ",\n\n" +
		"Welcome to Redirect! 🎉 Your account has been created successfully.\n\n" +
		"Here's how Redirect makes learning to code so much easier:\n\n" +
		"  • No more endless searching - pick a programming language and we point you straight to 10 of the best places to learn it.\n" +
		"  • Hand-picked resources: official docs, interactive tutorials, full courses and hands-on practice sites.\n" +
		"  • Your visits are saved, so you never lose your place.\n\n" +
		"Log in, tap Browse, choose a language, and start learning today.\n\n" +
		"Happy coding!\n" +
		"The Redirect Team\n"
	body = strings.ReplaceAll(body, "\n", "\r\n")

	msg := "From: Redirect <" + from + ">\r\n" +
		"To: " + toEmail + "\r\n" +
		"Subject: " + mime.QEncoding.Encode("utf-8", "Welcome to Redirect 🎉") + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" + body

	auth := smtp.PlainAuth("", user, pass, host)
	if err := smtp.SendMail(net.JoinHostPort(host, port), auth, from, []string{toEmail}, []byte(msg)); err != nil {
		log.Println("Could not send welcome email to", toEmail, ":", err)
		return
	}
	log.Println("Welcome email sent to", toEmail)
}
