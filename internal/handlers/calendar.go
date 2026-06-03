package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/aniwag2/theralert/internal/auth"
	"github.com/aniwag2/theralert/internal/models"
)

// Occurrence is a single rendered instance of an event on a given day.
type Occurrence struct {
	Event models.Event
	Start time.Time
}

// dayCell is one cell in the month grid.
type dayCell struct {
	Date        time.Time
	InMonth     bool
	IsToday     bool
	Occurrences []Occurrence
}

// Calendar renders a month grid of events for a group.
func (h *Handlers) Calendar(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	data := h.view(r)

	// Resolve which groups this user may view.
	var groups []models.Group
	if u.IsStaffSide() {
		groups, _ = h.Store.GroupsForOrg(r.Context(), u.OrgID)
	} else {
		groups, _ = h.Store.GroupsForUser(r.Context(), u.ID)
	}
	data["Groups"] = groups

	if len(groups) == 0 {
		data["NoGroups"] = true
		h.View.Render(w, http.StatusOK, "calendar", data)
		return
	}

	// Selected group (default first). Authorize access.
	selID, _ := strconv.ParseInt(r.URL.Query().Get("group"), 10, 64)
	var selected *models.Group
	for i := range groups {
		if groups[i].ID == selID {
			selected = &groups[i]
		}
	}
	if selected == nil {
		selected = &groups[0]
	}
	data["Selected"] = selected

	loc := h.Loc
	if loc == nil {
		loc = time.Local
	}

	// Month navigation (?y=&m=), default current month.
	now := time.Now().In(loc)
	year, _ := strconv.Atoi(r.URL.Query().Get("y"))
	month, _ := strconv.Atoi(r.URL.Query().Get("m"))
	if year == 0 || month < 1 || month > 12 {
		year, month = now.Year(), int(now.Month())
	}
	monthStart := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	monthEnd := monthStart.AddDate(0, 1, 0)

	// Grid spans whole weeks (Sun..Sat) covering the month.
	gridStart := monthStart.AddDate(0, 0, -int(monthStart.Weekday()))
	gridEnd := monthEnd
	if wd := monthEnd.Weekday(); wd != time.Sunday {
		gridEnd = monthEnd.AddDate(0, 0, int(7-wd))
	}

	events, _ := h.Store.EventsForCalendar(r.Context(), selected.ID, gridStart, gridEnd)
	occByDay := expandOccurrences(events, gridStart, gridEnd, loc)

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	var weeks [][]dayCell
	for d := gridStart; d.Before(gridEnd); d = d.AddDate(0, 0, 7) {
		week := make([]dayCell, 7)
		for i := 0; i < 7; i++ {
			day := d.AddDate(0, 0, i)
			key := day.Format("2006-01-02")
			week[i] = dayCell{
				Date:        day,
				InMonth:     day.Month() == time.Month(month),
				IsToday:     day.Equal(today),
				Occurrences: occByDay[key],
			}
		}
		weeks = append(weeks, week)
	}

	data["Weeks"] = weeks
	data["MonthLabel"] = monthStart.Format("January 2006")
	prev := monthStart.AddDate(0, 0, -1)
	next := monthEnd
	data["PrevY"], data["PrevM"] = prev.Year(), int(prev.Month())
	data["NextY"], data["NextM"] = next.Year(), int(next.Month())
	data["Today"] = now
	data["CanEdit"] = u.IsStaffSide()
	data["Recent"], _ = h.Store.RecentEvents(r.Context(), selected.ID, 8)

	h.View.Render(w, http.StatusOK, "calendar", data)
}

// expandOccurrences turns events (incl. weekly recurring) into per-day occurrences
// within [from,to), keyed by YYYY-MM-DD.
func expandOccurrences(events []models.Event, from, to time.Time, loc *time.Location) map[string][]Occurrence {
	out := make(map[string][]Occurrence)
	add := func(e models.Event, start time.Time) {
		if start.Before(from) || !start.Before(to) {
			return
		}
		key := start.In(loc).Format("2006-01-02")
		out[key] = append(out[key], Occurrence{Event: e, Start: start.In(loc)})
	}
	for _, e := range events {
		start := e.StartsAt.In(loc)
		switch e.Recurrence {
		case "weekly":
			// Advance to the first occurrence within the window, then step weekly.
			occ := start
			for occ.Before(from) {
				occ = occ.AddDate(0, 0, 7)
			}
			for occ.Before(to) {
				add(e, occ)
				occ = occ.AddDate(0, 0, 7)
			}
		default:
			add(e, start)
		}
	}
	return out
}

// CreateEvent logs an event (real-time activity, future one-time, or weekly recurring).
func (h *Handlers) CreateEvent(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	groupID, _ := strconv.ParseInt(r.FormValue("group_id"), 10, 64)

	// Authorize: staff/admin in org, or member of the group.
	g, err := h.Store.GroupByID(r.Context(), u.OrgID, groupID)
	if err != nil {
		http.Error(w, "Group not found", http.StatusNotFound)
		return
	}
	if !u.IsStaffSide() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	redirect := "/calendar?group=" + strconv.FormatInt(g.ID, 10)

	category := r.FormValue("category")
	if category != "therapy" && category != "activity" && category != "appointment" {
		category = "activity"
	}
	title := r.FormValue("title")
	if title == "" {
		http.Redirect(w, r, redirect+"&error="+urlEncode("Title is required."), http.StatusSeeOther)
		return
	}
	recurrence := r.FormValue("recurrence")
	if recurrence != "weekly" {
		recurrence = "none"
	}

	loc := h.Loc
	if loc == nil {
		loc = time.Local
	}
	// Determine start time: "now" for instant logging, else the provided datetime.
	var start time.Time
	if r.FormValue("when") == "now" || r.FormValue("datetime") == "" {
		start = time.Now()
	} else {
		// HTML datetime-local ("2006-01-02T15:04") is interpreted in the app timezone.
		t, perr := time.ParseInLocation("2006-01-02T15:04", r.FormValue("datetime"), loc)
		if perr != nil {
			http.Redirect(w, r, redirect+"&error="+urlEncode("Invalid date/time."), http.StatusSeeOther)
			return
		}
		start = t
	}

	staffID := u.ID
	e := &models.Event{
		OrgID:       u.OrgID,
		GroupID:     g.ID,
		Category:    category,
		Title:       title,
		Description: r.FormValue("description"),
		StartsAt:    start,
		Recurrence:  recurrence,
		CreatedBy:   &staffID,
	}
	if _, err := h.Store.CreateEvent(r.Context(), e); err != nil {
		http.Redirect(w, r, redirect+"&error="+urlEncode("Could not save event."), http.StatusSeeOther)
		return
	}

	// TODO(phase3): notify group recipients (email + SSE), honoring mute prefs.

	http.Redirect(w, r, redirect+"&flash="+urlEncode("Event added."), http.StatusSeeOther)
}

// DeleteEvent removes an event (staff/admin only).
func (h *Handlers) DeleteEvent(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	id, _ := strconv.ParseInt(r.FormValue("event_id"), 10, 64)
	groupID := r.FormValue("group_id")
	if err := h.Store.DeleteEvent(r.Context(), u.OrgID, id); err != nil {
		http.Redirect(w, r, "/calendar?group="+groupID+"&error="+urlEncode("Could not delete event."), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/calendar?group="+groupID+"&flash="+urlEncode("Event deleted."), http.StatusSeeOther)
}
