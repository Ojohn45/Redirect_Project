package main

import (
	"log"
	"net/http"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

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

func signupPageHandler(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "signup.html", nil)
}

func signupHandler(w http.ResponseWriter, r *http.Request) {
	fullName := r.FormValue("full_name")
	username := r.FormValue("username")
	password := r.FormValue("password")

	if fullName == "" || username == "" || password == "" {
		http.Error(w, "All fields are required", http.StatusBadRequest)
		return
	}

	if !isValidPassword(password) {
		http.Error(w, "Password must contain both letters and numbers", http.StatusBadRequest)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Println("Error hashing password:", err)
		return
	}

	_, err = db.Exec(
		"INSERT INTO users (full_name, username, password_hash) VALUES (?, ?, ?)",
		fullName, username, string(hashedPassword),
	)
	if err != nil {
		http.Error(w, "Username already taken", http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}