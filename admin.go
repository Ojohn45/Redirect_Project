package main

import (
	"errors"
	"log"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ---------- The head admin ----------
//
// The head admin is created automatically when the server starts.
// The PASSWORD is read from an environment variable so it never sits in
// your code (and never ends up on GitHub):
//
//   ADMIN_PASSWORD='...'  go run .
//
// The other details have defaults below, and can be changed with
// ADMIN_EMAIL, ADMIN_FULLNAME and ADMIN_USERNAME.
//
// This only happens while there is NO head admin yet. If an account with the
// admin email already exists, it is upgraded to head admin.

func seedHeadAdmin() {
	if _, exists := headAdmin(); exists {
		return // already there
	}

	password := os.Getenv("ADMIN_PASSWORD")
	if password == "" {
		log.Println("NOTE: no head admin yet. Start the server with ADMIN_PASSWORD set to create one.")
		return
	}

	email := strings.ToLower(getenv("ADMIN_EMAIL", "brucenic12@gmail.com"))
	fullName := getenv("ADMIN_FULLNAME", "Odumu John")
	username := getenv("ADMIN_USERNAME", "Ken")

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Println("Could not create head admin:", err)
		return
	}
	// If this email already has a normal account (for example from an earlier
	// test signup), upgrade THAT account instead of failing.
	if existing, found := findUserByEmail(email); found {
		if err := promoteToHeadAdmin(existing.ID, fullName, username, string(hash)); err != nil {
			log.Println("Could not make", existing.Username, "the head admin:", err, "(another account may already use the username", username+")")
			return
		}
		log.Printf("Existing account %q upgraded to head admin. Log in as %q with your ADMIN_PASSWORD.", existing.Username, username)
		return
	}

	if _, err := createUser(fullName, username, email, string(hash), RoleHeadAdmin); err != nil {
		log.Println("Could not create head admin:", err, "(is that username/email already used by another account?)")
		return
	}
	log.Println("Head admin created:", username)
}

// ---------- Guards: who may open which page ----------

// requireAdmin returns the user if they are an admin.
// Otherwise it redirects them away and returns nil.
func requireAdmin(w http.ResponseWriter, r *http.Request) *User {
	u := currentUser(r)
	if u == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return nil
	}
	if !u.IsAdmin() {
		http.Redirect(w, r, "/home", http.StatusSeeOther)
		return nil
	}
	return u
}

// Only the head admin may invite new admins.
func requireHeadAdmin(w http.ResponseWriter, r *http.Request) *User {
	u := requireAdmin(w, r)
	if u == nil {
		return nil
	}
	if !u.IsHeadAdmin() {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return nil
	}
	return u
}

// ---------- Admin pages ----------

func adminPageHandler(w http.ResponseWriter, r *http.Request) {
	u := requireAdmin(w, r)
	if u == nil {
		return
	}
	head, _ := headAdmin()
	render(w, "admin.html", PageData{User: u, HeadAdmin: &head})
}

func inviteAdminPageHandler(w http.ResponseWriter, r *http.Request) {
	u := requireHeadAdmin(w, r)
	if u == nil {
		return
	}
	render(w, "admin-invite.html", PageData{User: u})
}

func baseURL() string {
	return strings.TrimRight(getenv("BASE_URL", "http://localhost:8080"), "/")
}

func inviteAdminHandler(w http.ResponseWriter, r *http.Request) {
	u := requireHeadAdmin(w, r)
	if u == nil {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	fail := func(msg string) {
		w.WriteHeader(http.StatusBadRequest)
		render(w, "admin-invite.html", PageData{User: u, Error: msg, Email: email})
	}

	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		fail("Please enter a valid email address")
		return
	}
	if existing, ok := findUserByEmail(email); ok && existing.IsAdmin() {
		fail("That person is already an admin")
		return
	}

	token, err := createInvite(email, u.ID)
	if err != nil {
		log.Println("Could not save invite:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	link := baseURL() + "/admin/accept?token=" + token

	body := "Hello,\n\n" +
		u.FullName + " (Head Admin) has invited you to become an admin on Redirect.\n\n" +
		"As an admin you will help look after the Redirect community: support learners, keep an eye on how the platform is doing, " +
		"report problems early, and suggest new languages and learning resources so Redirect can keep growing.\n\n" +
		"Accept the invitation here (the link works for 7 days):\n" + link + "\n\n" +
		"If you weren't expecting this, you can safely ignore this email.\n\n" +
		"The Redirect Team\n"

	// Send in the background, but wait up to 15 seconds to tell the head admin how it went.
	done := make(chan error, 1)
	go func() { done <- sendEmail(email, "You're invited to be an admin on Redirect", body) }()
	var sendErr error
	select {
	case sendErr = <-done:
	case <-time.After(15 * time.Second):
		sendErr = errors.New("the email server took too long to answer")
	}

	data := PageData{User: u}
	switch {
	case sendErr == nil:
		data.Info = "Invitation sent to " + email + " ✅"
	case errors.Is(sendErr, errEmailNotConfigured):
		log.Println("Invite link for", email, ":", link)
		data.Info = "Email isn't set up on this server yet, so nothing was emailed. Send this link to " + email + " yourself:"
		data.InviteLink = link
	default:
		log.Println("Could not email invite to", email, ":", sendErr)
		data.Error = "The invitation was saved, but the email could not be sent. You can send this link yourself:"
		data.InviteLink = link
	}
	render(w, "admin-invite.html", data)
}

// A row in the "Check Users" table. Passwords are never included.
type UserRow struct {
	FullName string
	Username string
	Email    string
	Role     string
	Online   bool
}

func usersPageHandler(w http.ResponseWriter, r *http.Request) {
	u := requireAdmin(w, r)
	if u == nil {
		return
	}

	all := listUsers()
	rows := make([]UserRow, 0, len(all))
	online := 0
	for _, x := range all {
		on := isOnline(x.ID)
		if on {
			online++
		}
		rows = append(rows, UserRow{
			FullName: x.FullName,
			Username: x.Username,
			Email:    x.Email,
			Role:     x.RoleLabel(),
			Online:   on,
		})
	}
	render(w, "admin-users.html", PageData{User: u, UserRows: rows, TotalUsers: len(rows), OnlineUsers: online})
}

// ---------- Accepting an invitation (opened from the email link) ----------

func acceptPageData(r *http.Request, token string) (PageData, bool) {
	inv, ok := findInvite(token)
	if !ok {
		return PageData{Error: "This invitation link is invalid or has expired. Ask the head admin to send a new one."}, false
	}
	data := PageData{Token: token, InviteEmail: inv.Email}
	if existing, found := findUserByEmail(inv.Email); found {
		data.Existing = true
		if me := currentUser(r); me != nil && me.ID == existing.ID {
			data.CanAccept = true
		}
	}
	return data, true
}

func acceptAdminPageHandler(w http.ResponseWriter, r *http.Request) {
	data, ok := acceptPageData(r, r.URL.Query().Get("token"))
	data.User = currentUser(r)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
	}
	render(w, "accept-admin.html", data)
}

func acceptAdminHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	token := r.FormValue("token")

	data, ok := acceptPageData(r, token)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		render(w, "accept-admin.html", data)
		return
	}
	fail := func(msg string) {
		data.Error = msg
		data.FullName = strings.TrimSpace(r.FormValue("full_name"))
		data.Username = strings.TrimSpace(r.FormValue("username"))
		w.WriteHeader(http.StatusBadRequest)
		render(w, "accept-admin.html", data)
	}

	// Case 1: they already have an account and are logged into it -> upgrade it.
	if data.Existing {
		if !data.CanAccept {
			fail("An account with this email already exists. Please log in with it first, then open the invitation link again.")
			return
		}
		me := currentUser(r)
		if !me.IsAdmin() {
			if err := setUserRole(me.ID, RoleAdmin); err != nil {
				log.Println("Could not promote user:", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
		}
		markInviteUsed(token)
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	// Case 2: brand-new person -> create their admin account.
	fullName := strings.TrimSpace(r.FormValue("full_name"))
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")

	if fullName == "" || username == "" || password == "" {
		fail("All fields are required")
		return
	}
	if !isValidPassword(password) {
		fail("Password must contain both letters and numbers")
		return
	}
	if len(password) > 72 {
		fail("Password is too long (max 72 characters)")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	newUser, err := createUser(fullName, username, data.InviteEmail, string(hash), RoleAdmin)
	switch {
	case errors.Is(err, ErrUsernameTaken):
		fail("That username is already taken")
		return
	case errors.Is(err, ErrEmailTaken):
		fail("An account with this email already exists")
		return
	case err != nil:
		log.Println("Error saving admin:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	markInviteUsed(token)
	if err := startSession(w, newUser.ID); err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// ---------- Logout goodbye ----------

func goodbyePageHandler(w http.ResponseWriter, r *http.Request) {
	render(w, "goodbye.html", PageData{})
}
