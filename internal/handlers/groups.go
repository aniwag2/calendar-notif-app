package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/aniwag2/theralert/internal/auth"
	"github.com/aniwag2/theralert/internal/models"
)

// emailSplit splits a textarea of emails on commas, newlines, or whitespace.
var emailSep = func(r rune) bool {
	return r == ',' || r == '\n' || r == '\r' || r == ' ' || r == '\t' || r == ';'
}

// Groups renders the staff group-management page (list + create form).
func (h *Handlers) Groups(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	data := h.view(r)

	groups, _ := h.Store.GroupsForOrg(r.Context(), u.OrgID)
	// Attach members to each group for display.
	type groupView struct {
		models.Group
		Members []models.User
	}
	views := make([]groupView, 0, len(groups))
	for _, g := range groups {
		members, _ := h.Store.GroupMembers(r.Context(), g.ID)
		views = append(views, groupView{Group: g, Members: members})
	}
	data["Groups"] = views
	data["Patients"], _ = h.Store.UsersByOrg(r.Context(), u.OrgID, "patient")
	if msg := r.URL.Query().Get("flash"); msg != "" {
		data["Flash"] = msg
	}
	if msg := r.URL.Query().Get("error"); msg != "" {
		data["Error"] = msg
	}
	h.View.Render(w, http.StatusOK, "groups", data)
}

// CreateGroup creates a group with a patient and any number of family members (by email).
func (h *Handlers) CreateGroup(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	name := strings.TrimSpace(r.FormValue("name"))
	patientID, _ := strconv.ParseInt(r.FormValue("patient_id"), 10, 64)
	familyRaw := r.FormValue("family_emails")

	if name == "" {
		http.Redirect(w, r, "/groups?error="+urlEncode("Group name is required."), http.StatusSeeOther)
		return
	}

	// Validate patient belongs to this org and is a patient.
	var patientPtr *int64
	if patientID != 0 {
		p, err := h.Store.UserByID(r.Context(), patientID)
		if err != nil || p.OrgID != u.OrgID || p.Role != "patient" {
			http.Redirect(w, r, "/groups?error="+urlEncode("Selected patient is not valid."), http.StatusSeeOther)
			return
		}
		patientPtr = &patientID
	}

	staffID := u.ID
	g, err := h.Store.CreateGroup(r.Context(), u.OrgID, name, patientPtr, &staffID)
	if err != nil {
		http.Redirect(w, r, "/groups?error="+urlEncode("Could not create group."), http.StatusSeeOther)
		return
	}

	// Add family members by email; collect any that weren't found in this org.
	var notFound []string
	for _, raw := range strings.FieldsFunc(familyRaw, emailSep) {
		email := strings.ToLower(strings.TrimSpace(raw))
		if email == "" {
			continue
		}
		fam, err := h.Store.UserByEmail(r.Context(), email)
		if err != nil || fam.OrgID != u.OrgID {
			notFound = append(notFound, email)
			continue
		}
		_ = h.Store.AddGroupMember(r.Context(), g.ID, fam.ID)
	}

	msg := "Group \"" + g.Name + "\" created."
	if len(notFound) > 0 {
		msg += " These emails were not registered in your facility and were skipped: " + strings.Join(notFound, ", ")
	}
	http.Redirect(w, r, "/groups?flash="+urlEncode(msg), http.StatusSeeOther)
}

// DeleteGroup removes a group and its events/memberships.
func (h *Handlers) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	id, _ := strconv.ParseInt(r.FormValue("group_id"), 10, 64)
	if err := h.Store.DeleteGroup(r.Context(), u.OrgID, id); err != nil {
		http.Redirect(w, r, "/groups?error="+urlEncode("Could not delete group."), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/groups?flash="+urlEncode("Group deleted."), http.StatusSeeOther)
}
