# Theralert

A notification web app for assisted-living and nursing facilities. Staff log and
schedule patient activities; patients and their family members get notified by
email and in real time inside the app. **No personal medical information is
stored** — only name, email, and password.

Built as a single self-contained Go binary: server-rendered HTML
(`html/template`) + [HTMX](https://htmx.org) for live updates + Tailwind CSS,
backed by PostgreSQL. No Node.js runtime in production.

## Features

- **Organizations (multi-tenant):** each facility is isolated, with its own
  admins, staff, patients, and family members.
- **Roles:** admin, staff, patient, family — with separate staff and
  patient/family views.
- **Admin:** create staff accounts, promote/demote admins (the last admin can't
  be removed), and delete the organization (guarded, type-to-confirm).
- **Registration:** patients and family self-register with a facility code,
  optionally restricted to an allowed IP range.
- **Groups:** a patient plus any number of family members.
- **Events & calendar:** log activities in real time, schedule one-time future
  events, and create weekly-recurring events. Color-coded categories (therapy,
  activity, Dr. appointment) on a printable month calendar.
- **Notifications:** TZ-correct email plus live in-app updates (a bell badge and
  toasts via Server-Sent Events), with per-user selective muting (everything, by
  category, or by group).
- **Real-time clock** in the configured timezone.

## Configuration

Copy `.env.example` to `.env` and fill in the values:

| Variable        | Required | Description |
|-----------------|----------|-------------|
| `SESSION_SECRET`| yes      | Long random string for signing session cookies (`openssl rand -base64 32`). |
| `DATABASE_URL`  | yes\*    | Postgres connection string. Built from `DB_*` automatically in Docker Compose. |
| `BASE_URL`      | recommended | Public URL of the app. |
| `LOGOUT_URL`    | no       | Where the logout button redirects (default `https://theralert.aniwaghray.com`). |
| `SECURE_COOKIES`| no       | `true` when served over HTTPS (default `false`). |
| `TZ`            | no       | IANA timezone for clock/calendar/email, e.g. `America/New_York` (default UTC). |
| `ALLOWED_CIDRS` | no       | Comma-separated CIDRs/IPs allowed to register. Empty = allow all. |
| `SMTP_HOST`/`SMTP_PORT`/`SMTP_USER`/`SMTP_PASS`/`MAIL_FROM` | no | SMTP settings. If `SMTP_HOST` is empty, email is skipped (in-app notifications still work). |
| `DB_NAME`/`DB_USER`/`DB_PASSWORD` | Docker only | Used by Docker Compose to provision Postgres. |

\* `DATABASE_URL` is required when running the binary directly; Docker Compose
builds it from the `DB_*` variables.

## Running with Docker (recommended)

```bash
cp .env.example .env       # set SESSION_SECRET, DB_PASSWORD, BASE_URL, TZ, SMTP_*
docker compose up -d --build
```

This starts the app on port `3002` and a PostgreSQL container. The app runs
database migrations automatically on startup. Put it behind a reverse proxy
(Cloudflare Tunnel, Caddy, nginx, …) for HTTPS and a custom domain.

## Running locally (development)

Requires Go 1.25+ and a reachable PostgreSQL.

```bash
make run     # downloads the Tailwind CLI if needed, builds CSS, runs the server
```

`make run` reads `.env`. Point `DATABASE_URL` at your local Postgres, e.g.:

```
DATABASE_URL=postgres://theralert:devpass@localhost:5432/theralert?sslmode=disable
```

Other targets: `make build` (compile `./theralert`), `make css` /
`make css-watch` (rebuild Tailwind), `make tidy`.

## First run

With an empty database the app redirects to `/setup`, where you create the first
organization and its admin account. After that, staff/admins sign in at `/login`
and patients/family register at `/register` using the facility code.

## Project structure

```
cmd/server/          entrypoint + routes
internal/
  config/            env configuration
  db/                pgx pool + embedded migrations (db/migrations/*.sql)
  models/            data types + store (DB access)
  auth/              sessions, password hashing, role/auth middleware
  email/             SMTP sender (TZ-correct)
  realtime/          Server-Sent Events hub
  handlers/          HTTP handlers
web/
  render.go          template renderer (cache-busted CSS, TZ-aware helpers)
  templates/*.html   layout + pages
  static/            input.css -> app.css (Tailwind), htmx, favicon
tailwind.theralert.config.js   Tailwind config
Dockerfile, docker-compose.yml
```

## Tech stack

Go · chi (routing) · pgx (PostgreSQL) · gorilla/sessions · html/template · HTMX ·
Tailwind CSS (standalone CLI) · SSE · SMTP.
