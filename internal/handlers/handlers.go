package handlers

import (
	"context"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/aniwag2/theralert/internal/auth"
	"github.com/aniwag2/theralert/internal/config"
	"github.com/aniwag2/theralert/internal/email"
	"github.com/aniwag2/theralert/internal/models"
	"github.com/aniwag2/theralert/internal/realtime"
	"github.com/aniwag2/theralert/web"
)

var passwordRe = regexp.MustCompile(`[0-9]`)
var specialRe = regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>/?]`)

// Handlers bundles dependencies shared by all HTTP handlers.
type Handlers struct {
	Cfg   *config.Config
	Store *models.Store
	Auth  *auth.Manager
	View  *web.Renderer
	Hub   *realtime.Hub
	Email *email.Sender
	Loc   *time.Location
}

// view builds the common template data with the current user and their org.
func (h *Handlers) view(r *http.Request) map[string]any {
	data := map[string]any{}
	if u := auth.UserFrom(r.Context()); u != nil {
		data["User"] = u
		if org, err := h.Store.OrganizationByID(r.Context(), u.OrgID); err == nil {
			data["Org"] = org
		}
		if n, err := h.Store.UnreadCount(r.Context(), u.ID); err == nil {
			data["Unread"] = n
		}
	}
	return data
}

func validPassword(pw string) bool {
	return len(pw) >= 8 && passwordRe.MatchString(pw) && specialRe.MatchString(pw)
}

// clientIP extracts the best-effort client IP, honoring X-Forwarded-For.
func clientIP(r *http.Request) net.IP {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first := strings.TrimSpace(strings.Split(xff, ",")[0])
		if ip := net.ParseIP(first); ip != nil {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return net.ParseIP(host)
}

// --- Root / clock ---

func (h *Handlers) Root(w http.ResponseWriter, r *http.Request) {
	if auth.UserFrom(r.Context()) != nil {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// Clock returns the current time in the configured timezone as an HTMX fragment.
func (h *Handlers) Clock(w http.ResponseWriter, r *http.Request) {
	loc := h.Loc
	if loc == nil {
		loc = time.Local
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(time.Now().In(loc).Format("Mon Jan 2 · 3:04:05 PM MST")))
}

// --- First-run setup ---

func (h *Handlers) SetupForm(w http.ResponseWriter, r *http.Request) {
	if n, _ := h.Store.CountOrganizations(r.Context()); n > 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	h.View.Render(w, http.StatusOK, "setup", map[string]any{})
}

func (h *Handlers) Setup(w http.ResponseWriter, r *http.Request) {
	if n, _ := h.Store.CountOrganizations(r.Context()); n > 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	orgName := strings.TrimSpace(r.FormValue("orgname"))
	slug := slugify(r.FormValue("slug"))
	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")

	u, msg := h.createOrgAndAdmin(r.Context(), orgName, slug, name, email, password)
	if msg != "" {
		h.View.Render(w, http.StatusBadRequest, "setup", map[string]any{"Error": msg})
		return
	}
	_ = h.Auth.Login(w, r, u.ID)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// createOrgAndAdmin validates input and creates an organization plus its first
// admin user. It returns the new admin, or a user-facing error message.
func (h *Handlers) createOrgAndAdmin(ctx context.Context, orgName, slug, name, email, password string) (*models.User, string) {
	if orgName == "" || slug == "" || name == "" || email == "" {
		return nil, "All fields are required."
	}
	if !validPassword(password) {
		return nil, "Password must be at least 8 characters and include a number and a special character."
	}
	// Pre-checks avoid creating an org with no admin if the email/slug clashes.
	if exists, _ := h.Store.EmailExists(ctx, email); exists {
		return nil, "An account with this email already exists."
	}
	if _, err := h.Store.OrganizationBySlug(ctx, slug); err == nil {
		return nil, "That facility code is already taken. Please choose another."
	}
	org, err := h.Store.CreateOrganization(ctx, orgName, slug)
	if err != nil {
		return nil, "Could not create organization (is the facility code already taken?)."
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, "Internal error."
	}
	u, err := h.Store.CreateUser(ctx, org.ID, name, email, hash, "admin")
	if err != nil {
		return nil, "Could not create admin (is the email already in use?)."
	}
	return u, ""
}

// SignupForm renders the self-serve organization signup page.
func (h *Handlers) SignupForm(w http.ResponseWriter, r *http.Request) {
	if auth.UserFrom(r.Context()) != nil {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}
	h.View.Render(w, http.StatusOK, "signup", map[string]any{"Closed": !h.Cfg.SignupOpen})
}

// Signup creates a new organization and its first admin (self-serve).
func (h *Handlers) Signup(w http.ResponseWriter, r *http.Request) {
	if !h.Cfg.SignupOpen {
		h.View.Render(w, http.StatusForbidden, "signup", map[string]any{"Closed": true})
		return
	}
	orgName := strings.TrimSpace(r.FormValue("orgname"))
	slug := slugify(r.FormValue("slug"))
	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")

	u, msg := h.createOrgAndAdmin(r.Context(), orgName, slug, name, email, password)
	if msg != "" {
		h.View.Render(w, http.StatusBadRequest, "signup", map[string]any{
			"Error": msg, "OrgName": orgName, "Slug": slug, "Name": name, "Email": email,
		})
		return
	}
	_ = h.Auth.Login(w, r, u.ID)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// --- Auth: login / register / logout ---

func (h *Handlers) LoginForm(w http.ResponseWriter, r *http.Request) {
	if auth.UserFrom(r.Context()) != nil {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}
	if n, _ := h.Store.CountOrganizations(r.Context()); n == 0 {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	h.View.Render(w, http.StatusOK, "login", map[string]any{})
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")

	u, err := h.Store.UserByEmail(r.Context(), email)
	if err != nil || !auth.CheckPassword(u.PasswordHash, password) {
		h.View.Render(w, http.StatusUnauthorized, "login", map[string]any{
			"Error": "Invalid email or password.",
			"Email": email,
		})
		return
	}
	_ = h.Auth.Login(w, r, u.ID)
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func (h *Handlers) RegisterForm(w http.ResponseWriter, r *http.Request) {
	h.View.Render(w, http.StatusOK, "register", map[string]any{})
}

func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	// IP allowlist gate.
	if ip := clientIP(r); ip == nil || !h.Cfg.RegistrationAllowed(ip) {
		h.View.Render(w, http.StatusForbidden, "register", map[string]any{
			"Error": "Registration is not permitted from your network. Please contact your facility.",
		})
		return
	}

	orgSlug := slugify(r.FormValue("org"))
	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	role := r.FormValue("role")
	password := r.FormValue("password")

	form := map[string]any{"OrgSlug": orgSlug, "Name": name, "Email": email}
	fail := func(msg string) {
		form["Error"] = msg
		h.View.Render(w, http.StatusBadRequest, "register", form)
	}

	if role != "patient" && role != "family" {
		fail("Please choose patient or family.")
		return
	}
	org, err := h.Store.OrganizationBySlug(r.Context(), orgSlug)
	if err != nil {
		fail("Unknown facility code.")
		return
	}
	if !validPassword(password) {
		fail("Password must be at least 8 characters and include a number and a special character.")
		return
	}
	if exists, _ := h.Store.EmailExists(r.Context(), email); exists {
		fail("An account with this email already exists.")
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		fail("Internal error.")
		return
	}
	u, err := h.Store.CreateUser(r.Context(), org.ID, name, email, hash, role)
	if err != nil {
		fail("Could not create account.")
		return
	}
	_ = h.Auth.Login(w, r, u.ID)
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	_ = h.Auth.Logout(w, r)
	http.Redirect(w, r, h.Cfg.LogoutURL, http.StatusSeeOther)
}

// --- Dashboard ---

func (h *Handlers) Dashboard(w http.ResponseWriter, r *http.Request) {
	h.View.Render(w, http.StatusOK, "dashboard", h.view(r))
}

// --- Admin ---

func (h *Handlers) Admin(w http.ResponseWriter, r *http.Request) {
	data := h.view(r)
	u := auth.UserFrom(r.Context())
	users, _ := h.Store.UsersByOrg(r.Context(), u.OrgID, "")
	data["Users"] = users
	if msg := r.URL.Query().Get("flash"); msg != "" {
		data["Flash"] = msg
	}
	if msg := r.URL.Query().Get("error"); msg != "" {
		data["Error"] = msg
	}
	h.View.Render(w, http.StatusOK, "admin", data)
}

func (h *Handlers) CreateStaff(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")

	render := func(status int, extra map[string]any) {
		data := h.view(r)
		users, _ := h.Store.UsersByOrg(r.Context(), u.OrgID, "")
		data["Users"] = users
		for k, v := range extra {
			data[k] = v
		}
		h.View.Render(w, status, "admin", data)
	}

	if name == "" || email == "" {
		render(http.StatusBadRequest, map[string]any{"Error": "Name and email are required."})
		return
	}
	if !validPassword(password) {
		render(http.StatusBadRequest, map[string]any{"Error": "Password must be at least 8 characters and include a number and a special character."})
		return
	}
	if exists, _ := h.Store.EmailExists(r.Context(), email); exists {
		render(http.StatusBadRequest, map[string]any{"Error": "An account with this email already exists."})
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		render(http.StatusInternalServerError, map[string]any{"Error": "Internal error."})
		return
	}
	if _, err := h.Store.CreateUser(r.Context(), u.OrgID, name, email, hash, "staff"); err != nil {
		render(http.StatusInternalServerError, map[string]any{"Error": "Could not create staff account."})
		return
	}
	http.Redirect(w, r, "/admin?flash="+urlEncode("Staff account created for "+email), http.StatusSeeOther)
}

// --- Profile ---

func (h *Handlers) Profile(w http.ResponseWriter, r *http.Request) {
	data := h.view(r)
	if msg := r.URL.Query().Get("flash"); msg != "" {
		data["Flash"] = msg
	}
	h.View.Render(w, http.StatusOK, "profile", data)
}

func (h *Handlers) ChangePassword(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	current := r.FormValue("current")
	newPw := r.FormValue("new")

	fail := func(msg string) {
		data := h.view(r)
		data["Error"] = msg
		h.View.Render(w, http.StatusBadRequest, "profile", data)
	}
	if !auth.CheckPassword(u.PasswordHash, current) {
		fail("Current password is incorrect.")
		return
	}
	if !validPassword(newPw) {
		fail("New password must be at least 8 characters and include a number and a special character.")
		return
	}
	hash, err := auth.HashPassword(newPw)
	if err != nil {
		fail("Internal error.")
		return
	}
	if err := h.Store.UpdatePassword(r.Context(), u.ID, hash); err != nil {
		fail("Could not update password.")
		return
	}
	http.Redirect(w, r, "/profile?flash="+urlEncode("Password updated."), http.StatusSeeOther)
}

func (h *Handlers) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	if !auth.CheckPassword(u.PasswordHash, r.FormValue("password")) {
		data := h.view(r)
		data["Error"] = "Incorrect password. Account not deleted."
		h.View.Render(w, http.StatusUnauthorized, "profile", data)
		return
	}
	// Guard: the last admin cannot delete their account without breaking the org.
	if u.Role == "admin" {
		n, _ := h.Store.CountAdmins(r.Context(), u.OrgID)
		if n <= 1 {
			data := h.view(r)
			data["Error"] = "You are the only admin. Promote another admin first, or delete the organization from the Admin page."
			h.View.Render(w, http.StatusConflict, "profile", data)
			return
		}
	}
	if err := h.Store.DeleteUser(r.Context(), u.ID); err != nil {
		data := h.view(r)
		data["Error"] = "Could not delete account."
		h.View.Render(w, http.StatusInternalServerError, "profile", data)
		return
	}
	_ = h.Auth.Logout(w, r)
	http.Redirect(w, r, h.Cfg.LogoutURL, http.StatusSeeOther)
}

// --- Events (SSE) ---

func (h *Handlers) Events(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	h.Hub.ServeHTTP(w, r, u.ID)
}
