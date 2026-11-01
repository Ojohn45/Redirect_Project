package main

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	_ "embed" // lets us bake the banner picture into the program
	"encoding/base64"
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

// The animated dark-blue banner shown at the top of invitation emails.
// It is baked into the program, so it always travels WITH the email
// (no website needed to show it).
//
//go:embed static/email-banner.gif
var bannerGIF []byte

const bannerCID = "redirect-banner@redirect.local"

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

	// Plain text email (no HTML)
	if htmlBody == "" {
		header("Content-Type", "text/plain; charset=UTF-8")
		header("Content-Transfer-Encoding", "quoted-printable")
		buf.WriteString("\r\n")
		writeQP(&buf, textBody)
		return buf.Bytes(), nil
	}

	// Does the HTML use the animated banner? Then we attach it ("related").
	withBanner := strings.Contains(htmlBody, "cid:"+bannerCID)

	// Writes the "text version + pretty version" pair into w.
	writeAlternatives := func(w *multipart.Writer) error {
		for _, part := range []struct{ ctype, body string }{
			{"text/plain; charset=UTF-8", textBody}, // simple version first
			{"text/html; charset=UTF-8", htmlBody},  // pretty version last (mail apps prefer it)
		} {
			pw, err := w.CreatePart(textproto.MIMEHeader{
				"Content-Type":              {part.ctype},
				"Content-Transfer-Encoding": {"quoted-printable"},
			})
			if err != nil {
				return err
			}
			writeQP(pw, part.body)
		}
		return nil
	}

	if !withBanner {
		mw := multipart.NewWriter(&buf)
		header("Content-Type", "multipart/alternative; boundary="+mw.Boundary())
		buf.WriteString("\r\n")
		if err := writeAlternatives(mw); err != nil {
			return nil, err
		}
		return buf.Bytes(), mw.Close()
	}

	// related( alternative(text, html), image )
	outer := multipart.NewWriter(&buf)
	header("Content-Type", "multipart/related; type=\"multipart/alternative\"; boundary="+outer.Boundary())
	buf.WriteString("\r\n")

	// the text/html pair
	var altBuf bytes.Buffer
	inner := multipart.NewWriter(&altBuf)
	if err := writeAlternatives(inner); err != nil {
		return nil, err
	}
	if err := inner.Close(); err != nil {
		return nil, err
	}
	pw, err := outer.CreatePart(textproto.MIMEHeader{
		"Content-Type": {"multipart/alternative; boundary=" + inner.Boundary()},
	})
	if err != nil {
		return nil, err
	}
	pw.Write(altBuf.Bytes())

	// the animated picture
	iw, err := outer.CreatePart(textproto.MIMEHeader{
		"Content-Type":              {"image/gif; name=\"redirect-banner.gif\""},
		"Content-Transfer-Encoding": {"base64"},
		"Content-ID":                {"<" + bannerCID + ">"},
		"Content-Disposition":       {"inline; filename=\"redirect-banner.gif\""},
	})
	if err != nil {
		return nil, err
	}
	encoded := base64.StdEncoding.EncodeToString(bannerGIF)
	for len(encoded) > 0 { // email lines must be short: 76 characters each
		n := 76
		if len(encoded) < n {
			n = len(encoded)
		}
		iw.Write([]byte(encoded[:n] + "\r\n"))
		encoded = encoded[n:]
	}
	return buf.Bytes(), outer.Close()
}

// ---------- The emails themselves ----------

// A short description of the site, used in every invitation.
const redirectShort = "Redirect is a free launchpad for learning to code. Pick a programming language and we point you " +
	"straight to 10 of the best free places to learn it: official docs, interactive tutorials, full courses and practice sites."

// inviteEmail builds the dark-blue invitation email: an animated banner,
// a heading, ONE short paragraph that ends with the question "would you
// like to join?", and an Accept Invite button.
func inviteEmail(heading, message, link string) (text, htmlBody string) {
	text = heading + "\n\n" +
		message + "\n\n" +
		"ACCEPT INVITE:\n" + link + "\n\n" +
		"This invitation works for 7 days. If you weren't expecting it, you can safely ignore this email.\n\n" +
		"The Redirect Team\n"

	e := html.EscapeString
	var h strings.Builder
	h.WriteString(`<!DOCTYPE html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">` +
		`<meta name="color-scheme" content="dark"><meta name="supported-color-schemes" content="dark"></head>`)
	h.WriteString(`<body style="margin:0;padding:0;background:#030a18;" bgcolor="#030a18">`)
	h.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" bgcolor="#030a18" style="background:#030a18;"><tr><td align="center" style="padding:24px 12px;">`)
	h.WriteString(`<table role="presentation" width="600" cellpadding="0" cellspacing="0" bgcolor="#0a1a3a" style="max-width:600px;width:100%;background:#0a1a3a;border:1px solid #1e3a5c;border-radius:14px;">`)
	// animated banner
	h.WriteString(`<tr><td style="padding:0;line-height:0;font-size:0;border-radius:14px 14px 0 0;">` +
		`<img src="cid:` + bannerCID + `" width="600" alt="Redirect - Your gateway to great learning" ` +
		`style="display:block;width:100%;max-width:600px;height:auto;border:0;border-radius:14px 14px 0 0;color:#6cb2ff;font-size:22px;"></td></tr>`)
	h.WriteString(`<tr><td style="padding:32px 36px 12px;">`)
	h.WriteString(`<h1 style="margin:0 0 16px;color:#eef6ff;font-family:'Segoe UI',Arial,sans-serif;font-size:26px;line-height:1.3;">` + e(heading) + `</h1>`)
	h.WriteString(`<p style="margin:0;color:#c5d6ee;font-family:'Segoe UI',Arial,sans-serif;font-size:16px;line-height:1.7;">` + e(message) + `</p>`)
	h.WriteString(`</td></tr>`)
	// button
	h.WriteString(`<tr><td align="center" style="padding:24px 36px 28px;">` +
		`<table role="presentation" cellpadding="0" cellspacing="0"><tr><td align="center" bgcolor="#2b7de9" style="background:#2b7de9;border-radius:10px;">` +
		`<a href="` + e(link) + `" style="display:inline-block;padding:15px 44px;color:#ffffff;font-family:'Segoe UI',Arial,sans-serif;font-size:17px;font-weight:bold;text-decoration:none;border-radius:10px;">Accept Invite</a>` +
		`</td></tr></table></td></tr>`)
	// small print
	h.WriteString(`<tr><td style="padding:0 36px 30px;font-family:'Segoe UI',Arial,sans-serif;font-size:12px;line-height:1.6;color:#7f9bc0;">` +
		`If the button doesn't work, copy this link into your browser:<br>` +
		`<a href="` + e(link) + `" style="color:#6cb2ff;word-break:break-all;">` + e(link) + `</a><br><br>` +
		`This invitation works for 7 days. If you weren't expecting it, you can safely ignore this email.` +
		`</td></tr>`)
	h.WriteString(`</table></td></tr></table></body></html>`)
	return text, h.String()
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
