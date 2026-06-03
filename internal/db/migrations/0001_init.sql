-- Theralert initial schema (PostgreSQL). Multi-tenant from day one.
-- No personal medical information is stored anywhere.

CREATE TABLE IF NOT EXISTS organizations (
    id            BIGSERIAL PRIMARY KEY,
    name          TEXT NOT NULL,
    slug          TEXT NOT NULL UNIQUE,
    billing_status TEXT NOT NULL DEFAULT 'trial', -- trial | active | suspended
    allowed_cidrs TEXT NOT NULL DEFAULT '',       -- optional per-org override (comma separated)
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS users (
    id            BIGSERIAL PRIMARY KEY,
    org_id        BIGINT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    email         TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL CHECK (role IN ('admin','staff','patient','family')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (email)
);
CREATE INDEX IF NOT EXISTS idx_users_org ON users(org_id);

CREATE TABLE IF NOT EXISTS groups (
    id         BIGSERIAL PRIMARY KEY,
    org_id     BIGINT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    patient_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    staff_id   BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_groups_org ON groups(org_id);

CREATE TABLE IF NOT EXISTS group_members (
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, user_id)
);

-- events covers logged activities, one-time future events, and weekly recurring events.
CREATE TABLE IF NOT EXISTS events (
    id          BIGSERIAL PRIMARY KEY,
    org_id      BIGINT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    group_id    BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    category    TEXT NOT NULL DEFAULT 'activity' CHECK (category IN ('therapy','activity','appointment')),
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    starts_at   TIMESTAMPTZ NOT NULL,
    ends_at     TIMESTAMPTZ,
    recurrence  TEXT NOT NULL DEFAULT 'none' CHECK (recurrence IN ('none','weekly')),
    created_by  BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_events_group ON events(group_id);
CREATE INDEX IF NOT EXISTS idx_events_starts ON events(starts_at);

CREATE TABLE IF NOT EXISTS notifications (
    id         BIGSERIAL PRIMARY KEY,
    org_id     BIGINT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_id   BIGINT REFERENCES events(id) ON DELETE CASCADE,
    message    TEXT NOT NULL,
    read_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_notifications_user ON notifications(user_id, created_at DESC);

-- mute_prefs: selective notification muting. scope is 'all', a category, or 'group:<id>'.
CREATE TABLE IF NOT EXISTS mute_prefs (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scope   TEXT NOT NULL,
    muted   BOOLEAN NOT NULL DEFAULT true,
    PRIMARY KEY (user_id, scope)
);
