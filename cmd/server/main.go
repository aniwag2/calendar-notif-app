package main

import (
	"context"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/aniwag2/theralert/internal/auth"
	"github.com/aniwag2/theralert/internal/config"
	"github.com/aniwag2/theralert/internal/db"
	"github.com/aniwag2/theralert/internal/handlers"
	"github.com/aniwag2/theralert/internal/models"
	"github.com/aniwag2/theralert/internal/realtime"
	"github.com/aniwag2/theralert/web"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	database, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer database.Close()

	if err := database.Migrate(ctx); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	store := models.NewStore(database.Pool)
	authMgr := auth.NewManager(cfg.SessionSecret, cfg.SecureCookies, store)
	renderer, err := web.NewRenderer(cfg.Location)
	if err != nil {
		log.Fatalf("templates: %v", err)
	}
	hub := realtime.NewHub()

	h := &handlers.Handlers{Cfg: cfg, Store: store, Auth: authMgr, View: renderer, Hub: hub, Loc: cfg.Location}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(authMgr.LoadUser)

	// Static assets (embedded).
	staticFS, _ := fs.Sub(web.StaticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Public routes.
	r.Get("/", h.Root)
	r.Get("/clock", h.Clock)
	r.Get("/setup", h.SetupForm)
	r.Post("/setup", h.Setup)
	r.Get("/login", h.LoginForm)
	r.Post("/login", h.Login)
	r.Get("/register", h.RegisterForm)
	r.Post("/register", h.Register)
	r.Post("/logout", h.Logout)

	// Authenticated routes.
	r.Group(func(pr chi.Router) {
		pr.Use(auth.RequireAuth)
		pr.Get("/dashboard", h.Dashboard)
		pr.Get("/profile", h.Profile)
		pr.Post("/profile/password", h.ChangePassword)
		pr.Post("/profile/delete", h.DeleteAccount)
		pr.Get("/calendar", h.Calendar)
		pr.Get("/sse", h.Events)
	})

	// Staff/admin routes (groups + event management).
	r.Group(func(sr chi.Router) {
		sr.Use(auth.RequireAuth)
		sr.Use(auth.RequireRole("admin", "staff"))
		sr.Get("/groups", h.Groups)
		sr.Post("/groups", h.CreateGroup)
		sr.Post("/groups/delete", h.DeleteGroup)
		sr.Post("/events", h.CreateEvent)
		sr.Post("/events/delete", h.DeleteEvent)
	})

	// Admin-only routes.
	r.Group(func(ar chi.Router) {
		ar.Use(auth.RequireAuth)
		ar.Use(auth.RequireRole("admin"))
		ar.Get("/admin", h.Admin)
		ar.Post("/admin/staff", h.CreateStaff)
		ar.Post("/admin/role", h.SetRole)
		ar.Post("/admin/org/delete", h.DeleteOrg)
	})

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("Theralert listening on %s", cfg.Addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server: %v", err)
	}
}
