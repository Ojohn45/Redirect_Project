package main

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ---------- What we store ----------

type User struct {
	ID           int       `json:"id"`
	FullName     string    `json:"full_name"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"password_hash"`
	CreatedAt    time.Time `json:"created_at"`
}

type HistoryEntry struct {
	UserID       int       `json:"user_id"`
	LanguageSlug string    `json:"language_slug"`
	ResourceID   string    `json:"resource_id"`
	ResourceName string    `json:"resource_name"`
	ClickedAt    time.Time `json:"clicked_at"`
}

// ---------- Where we store it ----------

const storageDir = "storage"

var (
	usersFile   = filepath.Join(storageDir, "users.json")
	historyFile = filepath.Join(storageDir, "history.json")

	// One lock protects both slices below, so two requests
	// can never write to the files at the same moment.
	mu      sync.Mutex
	users   []User
	history []HistoryEntry
)

var (
	ErrUsernameTaken = errors.New("username already taken")
	ErrEmailTaken    = errors.New("email already registered")
)

// initDB creates the storage folder and loads the JSON files into memory.
func initDB() {
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		log.Fatal("Cannot create storage folder:", err)
	}
	if err := loadJSON(usersFile, &users); err != nil {
		log.Fatal("Cannot read users.json:", err)
	}
	if err := loadJSON(historyFile, &history); err != nil {
		log.Fatal("Cannot read history.json:", err)
	}
	log.Printf("Storage ready: %d user(s), %d history entries", len(users), len(history))
}

// loadJSON reads a file into v. A missing file is fine (first run).
func loadJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, v)
}

// saveJSON writes to a temp file first, then renames it over the real one.
// That way a crash mid-write can never leave a half-written (corrupt) file.
func saveJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ---------- Users ----------

func createUser(fullName, username, email, passwordHash string) (User, error) {
	mu.Lock()
	defer mu.Unlock()

	nextID := 1
	for _, u := range users {
		if strings.EqualFold(u.Username, username) {
			return User{}, ErrUsernameTaken
		}
		if strings.EqualFold(u.Email, email) {
			return User{}, ErrEmailTaken
		}
		if u.ID >= nextID {
			nextID = u.ID + 1
		}
	}

	u := User{
		ID:           nextID,
		FullName:     fullName,
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now(),
	}
	users = append(users, u)

	if err := saveJSON(usersFile, users); err != nil {
		users = users[:len(users)-1] // undo, so memory matches the file
		return User{}, err
	}
	return u, nil
}

func findUserByUsername(username string) (User, bool) {
	mu.Lock()
	defer mu.Unlock()
	for _, u := range users {
		if strings.EqualFold(u.Username, username) {
			return u, true
		}
	}
	return User{}, false
}

func findUserByID(id int) (User, bool) {
	mu.Lock()
	defer mu.Unlock()
	for _, u := range users {
		if u.ID == id {
			return u, true
		}
	}
	return User{}, false
}

// ---------- History ----------

func addHistory(userID int, slug, resourceID, resourceName string) error {
	mu.Lock()
	defer mu.Unlock()
	history = append(history, HistoryEntry{
		UserID:       userID,
		LanguageSlug: slug,
		ResourceID:   resourceID,
		ResourceName: resourceName,
		ClickedAt:    time.Now(),
	})
	if err := saveJSON(historyFile, history); err != nil {
		history = history[:len(history)-1]
		return err
	}
	return nil
}

// recentHistory returns the user's last n visits, newest first.
func recentHistory(userID, n int) []HistoryEntry {
	mu.Lock()
	defer mu.Unlock()
	var out []HistoryEntry
	for i := len(history) - 1; i >= 0 && len(out) < n; i-- {
		if history[i].UserID == userID {
			out = append(out, history[i])
		}
	}
	return out
}
