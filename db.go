package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
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
	Role         string    `json:"role,omitempty"` // "" = normal user, "admin", "head_admin"
	CreatedAt    time.Time `json:"created_at"`
}

const (
	RoleAdmin     = "admin"
	RoleHeadAdmin = "head_admin"
)

func (u User) IsAdmin() bool     { return u.Role == RoleAdmin || u.Role == RoleHeadAdmin }
func (u User) IsHeadAdmin() bool { return u.Role == RoleHeadAdmin }

func (u User) RoleLabel() string {
	switch u.Role {
	case RoleHeadAdmin:
		return "Head Admin"
	case RoleAdmin:
		return "Admin"
	}
	return "User"
}

// An invitation for someone to become an admin.
// We only keep a HASH of the secret token, like a password.
type Invite struct {
	TokenHash string    `json:"token_hash"`
	Email     string    `json:"email"`
	InvitedBy int       `json:"invited_by"`
	ForUser   bool      `json:"for_user,omitempty"` // true = invite a normal user; false = invite an admin
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
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
	invitesFile = filepath.Join(storageDir, "invites.json")

	// One lock protects both slices below, so two requests
	// can never write to the files at the same moment.
	mu      sync.Mutex
	users   []User
	history []HistoryEntry
	invites []Invite
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
	if err := loadJSON(invitesFile, &invites); err != nil {
		log.Fatal("Cannot read invites.json:", err)
	}
	seedHeadAdmin()
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

func createUser(fullName, username, email, passwordHash, role string) (User, error) {
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
		Role:         role,
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

// ---------- More user helpers ----------

func findUserByEmail(email string) (User, bool) {
	mu.Lock()
	defer mu.Unlock()
	for _, u := range users {
		if strings.EqualFold(u.Email, email) {
			return u, true
		}
	}
	return User{}, false
}

// listUsers returns a copy of every user (oldest first).
func listUsers() []User {
	mu.Lock()
	defer mu.Unlock()
	out := make([]User, len(users))
	copy(out, users)
	return out
}

func headAdmin() (User, bool) {
	mu.Lock()
	defer mu.Unlock()
	for _, u := range users {
		if u.Role == RoleHeadAdmin {
			return u, true
		}
	}
	return User{}, false
}

func setUserRole(id int, role string) error {
	mu.Lock()
	defer mu.Unlock()
	for i := range users {
		if users[i].ID == id {
			old := users[i].Role
			users[i].Role = role
			if err := saveJSON(usersFile, users); err != nil {
				users[i].Role = old
				return err
			}
			return nil
		}
	}
	return errors.New("user not found")
}

// ---------- Admin invites ----------

const inviteTTL = 7 * 24 * time.Hour

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// createInvite makes a secret token for this email. Any older unused
// invite for the same email is replaced. The token is returned ONCE
// (to put in the email link); only its hash is saved.
func createInvite(email string, invitedBy int, forUser bool) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)

	mu.Lock()
	defer mu.Unlock()

	backup := make([]Invite, len(invites))
	copy(backup, invites)

	kept := invites[:0]
	for _, inv := range invites {
		if !(strings.EqualFold(inv.Email, email) && !inv.Used) {
			kept = append(kept, inv)
		}
	}
	invites = append(kept, Invite{
		TokenHash: hashToken(token),
		Email:     email,
		InvitedBy: invitedBy,
		ForUser:   forUser,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(inviteTTL),
	})

	if err := saveJSON(invitesFile, invites); err != nil {
		invites = backup
		return "", err
	}
	return token, nil
}

// findInvite returns the invite only if it is unused and not expired.
func findInvite(token string) (Invite, bool) {
	h := hashToken(token)
	mu.Lock()
	defer mu.Unlock()
	for _, inv := range invites {
		if inv.TokenHash == h && !inv.Used && time.Now().Before(inv.ExpiresAt) {
			return inv, true
		}
	}
	return Invite{}, false
}

func markInviteUsed(token string) error {
	h := hashToken(token)
	mu.Lock()
	defer mu.Unlock()
	for i := range invites {
		if invites[i].TokenHash == h {
			invites[i].Used = true
			return saveJSON(invitesFile, invites)
		}
	}
	return nil
}

// promoteToHeadAdmin turns an EXISTING account into the head admin and
// sets its name, username and password hash. Used when the server starts.
func promoteToHeadAdmin(id int, fullName, username, passwordHash string) error {
	mu.Lock()
	defer mu.Unlock()

	idx := -1
	for i, u := range users {
		if u.ID == id {
			idx = i
		} else if strings.EqualFold(u.Username, username) {
			return ErrUsernameTaken // some OTHER account already uses that username
		}
	}
	if idx < 0 {
		return errors.New("user not found")
	}

	old := users[idx]
	users[idx].FullName = fullName
	users[idx].Username = username
	users[idx].PasswordHash = passwordHash
	users[idx].Role = RoleHeadAdmin

	if err := saveJSON(usersFile, users); err != nil {
		users[idx] = old
		return err
	}
	return nil
}
