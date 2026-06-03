// Package email sends notification emails over SMTP.
package email

import (
	"fmt"
	"net/smtp"
	"strings"
	"time"
)

// Sender sends HTML emails via SMTP (STARTTLS on submission ports).
type Sender struct {
	host string
	port string
	user string
	pass string
	from string
	loc  *time.Location
}

func New(host, port, user, pass, from string, loc *time.Location) *Sender {
	if loc == nil {
		loc = time.Local
	}
	if from == "" {
		from = user
	}
	return &Sender{host: host, port: port, user: user, pass: pass, from: from, loc: loc}
}

// Enabled reports whether SMTP is configured.
func (s *Sender) Enabled() bool { return s.host != "" }

// Loc exposes the configured timezone (used to format times in message bodies).
func (s *Sender) Loc() *time.Location { return s.loc }

// Send delivers an HTML email to one recipient. Safe to call from a goroutine.
func (s *Sender) Send(to, subject, htmlBody string) error {
	if !s.Enabled() {
		return nil
	}
	addr := s.host + ":" + s.port
	var auth smtp.Auth
	if s.user != "" {
		auth = smtp.PlainAuth("", s.user, s.pass, s.host)
	}

	date := time.Now().In(s.loc).Format(time.RFC1123Z)
	var msg strings.Builder
	fmt.Fprintf(&msg, "From: %s\r\n", s.from)
	fmt.Fprintf(&msg, "To: %s\r\n", to)
	fmt.Fprintf(&msg, "Subject: %s\r\n", subject)
	fmt.Fprintf(&msg, "Date: %s\r\n", date)
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(htmlBody)

	return smtp.SendMail(addr, auth, s.from, []string{to}, []byte(msg.String()))
}
