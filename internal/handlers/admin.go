package handlers

import (
	"net/http"
	"strconv"

	"github.com/aniwag2/theralert/internal/auth"
)

// SetRole promotes a staff member to admin or demotes an admin to staff.
// Guards against removing the last admin in the org.
func (h *Handlers) SetRole(w http.ResponseWriter, r *http.Request) {
	actor := auth.UserFrom(r.Context())
	targetID, _ := strconv.ParseInt(r.FormValue("user_id"), 10, 64)
	newRole := r.FormValue("role")

	if newRole != "admin" && newRole != "staff" {
		http.Redirect(w, r, "/admin?error="+urlEncode("Role can only be set to admin or staff."), http.StatusSeeOther)
		return
	}

	target, err := h.Store.UserByID(r.Context(), targetID)
	if err != nil || target.OrgID != actor.OrgID {
		http.Redirect(w, r, "/admin?error="+urlEncode("User not found in your organization."), http.StatusSeeOther)
		return
	}
	// Only staff<->admin transitions are allowed here (not patients/family).
	if target.Role != "admin" && target.Role != "staff" {
		http.Redirect(w, r, "/admin?error="+urlEncode("Only staff and admins can change role."), http.StatusSeeOther)
		return
	}
	// Prevent demoting the last admin.
	if target.Role == "admin" && newRole == "staff" {
		n, _ := h.Store.CountAdmins(r.Context(), actor.OrgID)
		if n <= 1 {
			http.Redirect(w, r, "/admin?error="+urlEncode("You cannot demote the last admin. Promote another admin first."), http.StatusSeeOther)
			return
		}
	}
	if err := h.Store.SetRole(r.Context(), target.ID, newRole); err != nil {
		http.Redirect(w, r, "/admin?error="+urlEncode("Could not update role."), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin?flash="+urlEncode(target.Name+" is now "+newRole+"."), http.StatusSeeOther)
}

// DeleteOrg permanently deletes the admin's organization and all its data.
func (h *Handlers) DeleteOrg(w http.ResponseWriter, r *http.Request) {
	actor := auth.UserFrom(r.Context())
	org, err := h.Store.OrganizationByID(r.Context(), actor.OrgID)
	if err != nil {
		http.Redirect(w, r, "/admin?error="+urlEncode("Organization not found."), http.StatusSeeOther)
		return
	}
	confirm := r.FormValue("confirm")
	if confirm != org.Slug {
		http.Redirect(w, r, "/admin?error="+urlEncode("Type the facility code exactly to confirm deletion."), http.StatusSeeOther)
		return
	}
	if err := h.Store.DeleteOrganization(r.Context(), org.ID); err != nil {
		http.Redirect(w, r, "/admin?error="+urlEncode("Could not delete organization."), http.StatusSeeOther)
		return
	}
	_ = h.Auth.Logout(w, r)
	http.Redirect(w, r, "/setup", http.StatusSeeOther)
}
