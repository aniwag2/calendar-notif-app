package handlers

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/aniwag2/theralert/internal/models"
)

// categoryLabel returns a human-friendly category name.
func categoryLabel(cat string) string {
	switch cat {
	case "therapy":
		return "Therapy"
	case "appointment":
		return "Dr. appointment"
	default:
		return "Activity"
	}
}

// notifyGroup notifies a group's patient and family members about an event,
// across in-app (persisted + live SSE) and email channels, honoring each
// recipient's mute preferences. Emails are sent asynchronously.
func (h *Handlers) notifyGroup(ctx context.Context, e *models.Event, g *models.Group) {
	loc := h.Loc
	if loc == nil {
		loc = time.Local
	}

	recipients, err := h.Store.GroupRecipients(ctx, g.ID)
	if err != nil {
		return
	}

	when := e.StartsAt.In(loc).Format("Mon Jan 2 at 3:04 PM MST")
	label := categoryLabel(e.Category)
	repeat := ""
	if e.Recurrence == "weekly" {
		repeat = " (repeats weekly)"
	}

	// Single-line message for in-app + SSE.
	msg := fmt.Sprintf("%s: %s — %s%s", label, e.Title, when, repeat)

	subject := fmt.Sprintf("Theralert: %s for %s", label, groupName(g))
	body := buildEmailHTML(label, e, g, when, repeat)

	for _, rcpt := range recipients {
		prefs, _ := h.Store.MutePrefs(ctx, rcpt.ID)
		if models.IsMuted(prefs, e.Category, g.ID) {
			continue
		}
		eventID := e.ID
		// Persist the in-app notification.
		_, _ = h.Store.CreateNotification(ctx, e.OrgID, rcpt.ID, &eventID, msg)
		// Live push to any open browser sessions.
		h.Hub.Notify(rcpt.ID, "notify", msg)
		// Email (async; uses a detached context implicitly via net/smtp).
		if h.Email != nil && h.Email.Enabled() {
			to := rcpt.Email
			go func() {
				_ = h.Email.Send(to, subject, body)
			}()
		}
	}
}

func groupName(g *models.Group) string {
	if g.PatientName != "" {
		return g.PatientName
	}
	return g.Name
}

func buildEmailHTML(label string, e *models.Event, g *models.Group, when, repeat string) string {
	color := "#16a34a"
	switch e.Category {
	case "therapy":
		color = "#2563eb"
	case "appointment":
		color = "#db2777"
	}
	desc := ""
	if strings.TrimSpace(e.Description) != "" {
		desc = fmt.Sprintf(`<p style="margin:8px 0;"><strong>Details:</strong> %s</p>`, html.EscapeString(e.Description))
	}
	return fmt.Sprintf(`<div style="font-family:Arial,sans-serif;line-height:1.6;color:#333;max-width:560px;">
  <h2 style="color:%s;margin-bottom:4px;">%s</h2>
  <p style="color:#666;margin-top:0;">For %s</p>
  <p style="margin:8px 0;"><strong>%s</strong>%s</p>
  <p style="margin:8px 0;"><strong>When:</strong> %s</p>
  %s
  <p style="margin-top:16px;">Open Theralert to see the full calendar.</p>
  <hr style="border:0;border-top:1px solid #eee;margin:20px 0;">
  <p style="font-size:0.85em;color:#999;">You're receiving this because you're part of this group on Theralert.
  You can mute notifications anytime from your Notifications page. This is an automated message; please don't reply.</p>
</div>`,
		color, label, html.EscapeString(groupName(g)),
		html.EscapeString(e.Title), repeat,
		html.EscapeString(when), desc)
}
