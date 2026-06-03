package auth

import (
	"context"
	"net/http"

	"github.com/aniwag2/theralert/internal/models"
	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

const sessionName = "theralert_session"

type ctxKey int

const userCtxKey ctxKey = 0

// Manager handles password hashing, session cookies, and request auth.
type Manager struct {
	store  *sessions.CookieStore
	users  *models.Store
	secure bool
}

func NewManager(secret []byte, secure bool, users *models.Store) *Manager {
	cs := sessions.NewCookieStore(secret)
	cs.Options = &sessions.Options{
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   60 * 60 * 24 * 7, // 1 week
	}
	return &Manager{store: cs, users: users, secure: secure}
}

func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(b), err
}

func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// Login writes the authenticated user id into the session cookie.
func (m *Manager) Login(w http.ResponseWriter, r *http.Request, userID int64) error {
	sess, _ := m.store.Get(r, sessionName)
	sess.Values["uid"] = userID
	return sess.Save(r, w)
}

// Logout clears the session.
func (m *Manager) Logout(w http.ResponseWriter, r *http.Request) error {
	sess, _ := m.store.Get(r, sessionName)
	sess.Options.MaxAge = -1
	delete(sess.Values, "uid")
	return sess.Save(r, w)
}

// currentUserID returns the user id stored in the session, if any.
func (m *Manager) currentUserID(r *http.Request) (int64, bool) {
	sess, err := m.store.Get(r, sessionName)
	if err != nil {
		return 0, false
	}
	uid, ok := sess.Values["uid"].(int64)
	return uid, ok && uid != 0
}

// LoadUser is middleware that attaches the logged-in user (if any) to the context.
func (m *Manager) LoadUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if uid, ok := m.currentUserID(r); ok {
			if u, err := m.users.UserByID(r.Context(), uid); err == nil {
				r = r.WithContext(context.WithValue(r.Context(), userCtxKey, u))
			}
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAuth redirects to /login when no user is present.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserFrom(r.Context()) == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireRole restricts access to users whose role is in the allowed set.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := UserFrom(r.Context())
			if u == nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			if !allowed[u.Role] {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// UserFrom extracts the current user from a request context, or nil.
func UserFrom(ctx context.Context) *models.User {
	u, _ := ctx.Value(userCtxKey).(*models.User)
	return u
}
