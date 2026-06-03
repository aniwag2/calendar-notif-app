package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/aniwag2/theralert/internal/auth"
)

// muteScope is one toggle row on the notifications settings form.
type muteScope struct {
	Scope string
	Label string
	Muted bool
}

// Notifications renders the user's notification feed and mute settings.
func (h *Handlers) Notifications(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	data := h.view(r)

	notifs, _ := h.Store.NotificationsForUser(r.Context(), u.ID, 50)
	data["Notifications"] = notifs

	prefs, _ := h.Store.MutePrefs(r.Context(), u.ID)
	scopes := []muteScope{
		{Scope: "all", Label: "Mute everything", Muted: prefs["all"]},
		{Scope: "therapy", Label: "Therapy", Muted: prefs["therapy"]},
		{Scope: "activity", Label: "Activities", Muted: prefs["activity"]},
		{Scope: "appointment", Label: "Dr. appointments", Muted: prefs["appointment"]},
	}
	// Per-group mute toggles for groups the user belongs to.
	groups, _ := h.Store.GroupsForUser(r.Context(), u.ID)
	for _, g := range groups {
		scope := "group:" + strconv.FormatInt(g.ID, 10)
		scopes = append(scopes, muteScope{Scope: scope, Label: "Group: " + g.Name, Muted: prefs[scope]})
	}
	data["MuteScopes"] = scopes

	if msg := r.URL.Query().Get("flash"); msg != "" {
		data["Flash"] = msg
	}
	h.View.Render(w, http.StatusOK, "notifications", data)
}

// MarkRead marks all of the user's notifications as read.
func (h *Handlers) MarkRead(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	_ = h.Store.MarkAllRead(r.Context(), u.ID)
	http.Redirect(w, r, "/notifications", http.StatusSeeOther)
}

// SetMutes saves the user's mute preferences. Any known scope not checked is unmuted.
func (h *Handlers) SetMutes(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	_ = r.ParseForm()

	// Build the full set of known scopes for this user.
	scopes := []string{"all", "therapy", "activity", "appointment"}
	groups, _ := h.Store.GroupsForUser(r.Context(), u.ID)
	for _, g := range groups {
		scopes = append(scopes, "group:"+strconv.FormatInt(g.ID, 10))
	}

	checked := make(map[string]bool)
	for _, s := range r.Form["mute"] {
		checked[s] = true
	}
	for _, scope := range scopes {
		_ = h.Store.SetMute(r.Context(), u.ID, scope, checked[scope])
	}
	http.Redirect(w, r, "/notifications?flash="+urlEncode("Notification settings saved."), http.StatusSeeOther)
}

// NotifCount returns the unread-count badge as an HTMX fragment.
func (h *Handlers) NotifCount(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	n, _ := h.Store.UnreadCount(r.Context(), u.ID)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(badgeHTML(n)))
}

func badgeHTML(n int) string {
	if n <= 0 {
		return ""
	}
	label := strconv.Itoa(n)
	if n > 9 {
		label = "9+"
	}
	return fmt.Sprintf(`<span class="absolute -top-1 -right-1 inline-flex h-4 min-w-[1rem] items-center justify-center rounded-full bg-red-500 px-1 text-[10px] font-bold text-white">%s</span>`, label)
}
