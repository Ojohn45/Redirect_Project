package main

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"html"
	"log"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"net/textproto"
	"os"
	"strings"
	"time"
)

// ---------- Email settings ----------
//
// Put these in a file called .env next to main.go (see .env.example):
//   SMTP_USER  your Gmail address
//   SMTP_PASS  a Gmail "App Password" (NOT your normal password)
// Optional: SMTP_HOST (default smtp.gmail.com), SMTP_PORT (default 587), SMTP_FROM

var errEmailNotConfigured = errors.New("email is not configured (SMTP_USER / SMTP_PASS missing)")

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loadDotEnv reads a ".env" file (if there is one) and sets each line as an
// environment variable. Real environment variables win over the file.
func loadDotEnv() {
	data, err := os.ReadFile(".env")
	if err != nil {
		wd, _ := os.Getwd()
		log.Println("No .env file found in", wd)
		// A very common mix-up: the file exists but has the wrong name.
		for _, wrong := range []string{"set.env", "env", ".env.txt", "env.txt", ".env.example"} {
			if _, statErr := os.Stat(wrong); statErr == nil {
				log.Printf("HINT: I found %q - rename it to exactly .env (a dot, then env, nothing else)", wrong)
				break
			}
		}
		return
	}
	loaded := 0
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1] // remove surrounding quotes
		}
		if key == "" || value == "" || os.Getenv(key) != "" {
			continue
		}
		os.Setenv(key, value)
		loaded++
	}
	log.Printf("Loaded %d setting(s) from .env", loaded)
}

// logEmailStatus tells you right at startup whether email is ready.
func logEmailStatus() {
	user := os.Getenv("SMTP_USER")
	pass := os.Getenv("SMTP_PASS")
	switch {
	case user == "" && pass == "":
		log.Println("EMAIL: NOT set up - add SMTP_USER and SMTP_PASS to your .env file, then restart")
	case user == "" || pass == "":
		log.Println("EMAIL: only one of SMTP_USER / SMTP_PASS is set - you need both")
	default:
		log.Println("EMAIL: settings found, testing the login with Gmail...")
		go checkEmailLogin() // runs in the background, result appears in a few seconds
	}
}

// checkEmailLogin connects to the mail server and logs in, WITHOUT sending
// anything, so you find out at startup if the Gmail details are right.
func checkEmailLogin() {
	user := os.Getenv("SMTP_USER")
	pass := strings.ReplaceAll(os.Getenv("SMTP_PASS"), " ", "")
	host := getenv("SMTP_HOST", "smtp.gmail.com")
	port := getenv("SMTP_PORT", "587")

	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 10*time.Second)
	if err != nil {
		log.Println("EMAIL CHECK: could not reach", host, "- is your internet working?", err)
		return
	}
	conn.SetDeadline(time.Now().Add(15 * time.Second)) // never wait forever
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		log.Println("EMAIL CHECK: the mail server did not answer properly:", err)
		return
	}
	defer c.Close()

	if err := c.Hello("localhost"); err != nil {
		log.Println("EMAIL CHECK: the mail server did not answer properly:", err)
		return
	}
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: host}); err != nil {
			log.Println("EMAIL CHECK: could not make a secure connection:", err)
			return
		}
	}
	if err := c.Auth(smtp.PlainAuth("", user, pass, host)); err != nil {
		log.Println("EMAIL CHECK: FAILED - the mail server refused the login for", user)
		log.Println("   reason:", err)
		log.Println("   FIX: SMTP_USER must be the SAME Gmail account that created the App Password,")
		log.Println("        and SMTP_PASS must be that 16-letter App Password (not your normal password).")
		return
	}
	c.Quit()
	log.Println("EMAIL CHECK: OK - login accepted, invitations will be emailed from", user)
}

// ---------- Sending ----------

// sendEmail sends an email. If htmlBody is empty it is plain text; otherwise
// it is a "text + pretty HTML" email (mail apps show the HTML version).
// toEmail must already be validated.
func sendEmail(toEmail, subject, textBody, htmlBody string) error {
	user := os.Getenv("SMTP_USER")
	pass := strings.ReplaceAll(os.Getenv("SMTP_PASS"), " ", "") // Google shows App Passwords with spaces
	if user == "" || pass == "" {
		return errEmailNotConfigured
	}

	host := getenv("SMTP_HOST", "smtp.gmail.com")
	port := getenv("SMTP_PORT", "587")
	from := getenv("SMTP_FROM", user)

	msg, err := buildMessage(from, toEmail, subject, textBody, htmlBody)
	if err != nil {
		return err
	}

	auth := smtp.PlainAuth("", user, pass, host)
	return smtp.SendMail(net.JoinHostPort(host, port), auth, from, []string{toEmail}, msg)
}

func writeQP(w interface{ Write([]byte) (int, error) }, s string) {
	qp := quotedprintable.NewWriter(w)
	qp.Write([]byte(s))
	qp.Close()
}

func buildMessage(from, to, subject, textBody, htmlBody string) ([]byte, error) {
	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	header := func(k, v string) { buf.WriteString(k + ": " + v + "\r\n") }
	header("From", "Redirect <"+from+">")
	header("To", to)
	header("Subject", mime.QEncoding.Encode("utf-8", subject))
	header("Date", time.Now().Format(time.RFC1123Z))
	header("Message-ID", "<"+hex.EncodeToString(idBytes)+"@redirect.local>")
	header("MIME-Version", "1.0")

	if htmlBody == "" {
		header("Content-Type", "text/plain; charset=UTF-8")
		header("Content-Transfer-Encoding", "quoted-printable")
		buf.WriteString("\r\n")
		writeQP(&buf, textBody)
		return buf.Bytes(), nil
	}

	mw := multipart.NewWriter(&buf)
	header("Content-Type", "multipart/alternative; boundary="+mw.Boundary())
	buf.WriteString("\r\n")

	for _, part := range []struct{ ctype, body string }{
		{"text/plain; charset=UTF-8", textBody}, // simple version first
		{"text/html; charset=UTF-8", htmlBody},  // pretty version last (mail apps prefer it)
	} {
		pw, err := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {part.ctype},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		if err != nil {
			return nil, err
		}
		writeQP(pw, part.body)
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ---------- The emails themselves ----------

const redirectAbout = "Redirect isn't another course platform trying to teach you everything from scratch. " +
	"It's a curated launchpad. Pick a programming language you want to learn, and Redirect instantly points you " +
	"to 10 of the best free resources on the web for it: official docs, interactive tutorials, full courses, " +
	"and hands-on practice sites. No more spending an hour searching \"best way to learn Python\". " +
	"Just pick a language, click, and start learning. Your account keeps track of every resource you've " +
	"visited, so you never lose your place."

// inviteEmail builds the invitation email: a short intro, a paragraph about
// what Redirect is, (optionally) an extra paragraph, and an Accept Invite button.
func inviteEmail(inviterName, intro, extra, link string) (text, htmlBody string) {
	var t strings.Builder
	t.WriteString("Hello,\n\n")
	t.WriteString(inviterName + " " + intro + "\n\n")
	t.WriteString("WHAT IS REDIRECT?\n" + redirectAbout + "\n\n")
	if extra != "" {
		t.WriteString(extra + "\n\n")
	}
	t.WriteString("ACCEPT INVITE:\n" + link + "\n\n")
	t.WriteString("This invitation works for 7 days. If you weren't expecting it, you can safely ignore this email.\n\n")
	t.WriteString("The Redirect Team\n")

	e := html.EscapeString
	var h strings.Builder
	h.WriteString(`<!DOCTYPE html><html><body style="margin:0;padding:0;background:#eef3f9;font-family:'Segoe UI',Arial,sans-serif;">`)
	h.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#eef3f9;padding:24px 12px;"><tr><td align="center">`)
	h.WriteString(`<table role="presentation" width="560" cellpadding="0" cellspacing="0" style="max-width:560px;width:100%;background:#ffffff;border-radius:12px;border:1px solid #d6e2f0;">`)
	h.WriteString(`<tr><td style="padding:32px;">`)
	h.WriteString(`<h1 style="margin:0;color:#2b7de9;font-size:28px;">Redirect</h1>`)
	h.WriteString(`<p style="margin:4px 0 24px;color:#5b7a99;">Your gateway to great learning.</p>`)
	h.WriteString(`<p style="color:#1a2b3c;line-height:1.6;font-size:16px;">Hello,<br><br><strong>` + e(inviterName) + `</strong> ` + e(intro) + `</p>`)
	h.WriteString(`<h3 style="margin:24px 0 8px;color:#2b7de9;font-size:17px;">What is Redirect?</h3>`)
	h.WriteString(`<p style="color:#1a2b3c;line-height:1.7;font-size:15px;margin:0;">` + e(redirectAbout) + `</p>`)
	if extra != "" {
		h.WriteString(`<p style="color:#1a2b3c;line-height:1.7;font-size:15px;margin:16px 0 0;">` + e(extra) + `</p>`)
	}
	h.WriteString(`<p style="text-align:center;margin:32px 0;"><a href="` + e(link) + `" style="display:inline-block;background:#2b7de9;color:#ffffff;text-decoration:none;font-weight:bold;font-size:16px;padding:14px 36px;border-radius:8px;">Accept Invite</a></p>`)
	h.WriteString(`<p style="color:#7a93ad;font-size:12px;line-height:1.5;margin:0;">If the button doesn't work, copy this link into your browser:<br><a href="` + e(link) + `" style="color:#2b7de9;word-break:break-all;">` + e(link) + `</a></p>`)
	h.WriteString(`<p style="color:#7a93ad;font-size:12px;margin:16px 0 0;">This invitation works for 7 days. If you weren't expecting it, you can safely ignore this email.</p>`)
	h.WriteString(`</td></tr></table></td></tr></table></body></html>`)
	return t.String(), h.String()
}

func sendWelcomeEmail(toEmail, fullName string) {
	name := fullName
	if parts := strings.Fields(fullName); len(parts) > 0 {
		name = parts[0]
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

	err := sendEmail(toEmail, "Welcome to Redirect 🎉", body, "")
	switch {
	case errors.Is(err, errEmailNotConfigured):
		log.Println("Email not configured - skipping welcome email to", toEmail)
	case err != nil:
		log.Println("Could not send welcome email to", toEmail, ":", err)
	default:
		log.Println("Welcome email sent to", toEmail)
	}
}
