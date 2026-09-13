package main

import (
	"database/sql"
	"log"
	"unicode"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func initDB() {
	var err error
	db, err = sql.Open("sqlite3", "redirect.db")
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}

	createTables := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		full_name TEXT NOT NULL,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		language_slug TEXT NOT NULL,
		resource_id TEXT NOT NULL,
		resource_name TEXT NOT NULL,
		clicked_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);
	`

	_, err = db.Exec(createTables)
	if err != nil {
		log.Fatal("Failed to create tables:", err)
	}

	log.Println("Database initialized successfully")
}

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
