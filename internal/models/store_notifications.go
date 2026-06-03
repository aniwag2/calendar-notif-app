package models

import (
	"context"
	"strconv"
)

// --- Notifications ---

// CreateNotification records an in-app notification for a user.
func (s *Store) CreateNotification(ctx context.Context, orgID, userID int64, eventID *int64, message string) (*Notification, error) {
	n := &Notification{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO notifications (org_id, user_id, event_id, message)
		 VALUES ($1,$2,$3,$4)
		 RETURNING id, org_id, user_id, event_id, message, read_at, created_at`,
		orgID, userID, eventID, message,
	).Scan(&n.ID, &n.OrgID, &n.UserID, &n.EventID, &n.Message, &n.ReadAt, &n.CreatedAt)
	if err != nil {
		return nil, err
	}
	return n, nil
}

// NotificationsForUser returns a user's most recent notifications.
func (s *Store) NotificationsForUser(ctx context.Context, userID int64, limit int) ([]Notification, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, org_id, user_id, event_id, message, read_at, created_at
		 FROM notifications WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.OrgID, &n.UserID, &n.EventID, &n.Message, &n.ReadAt, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// UnreadCount returns the number of unread notifications for a user.
func (s *Store) UnreadCount(ctx context.Context, userID int64) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL`, userID).Scan(&n)
	return n, err
}

// MarkAllRead marks all of a user's notifications as read.
func (s *Store) MarkAllRead(ctx context.Context, userID int64) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE notifications SET read_at = now() WHERE user_id = $1 AND read_at IS NULL`, userID)
	return err
}

// --- Mute preferences ---

// MutePrefs returns a user's mute settings as scope -> muted.
func (s *Store) MutePrefs(ctx context.Context, userID int64) (map[string]bool, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT scope, muted FROM mute_prefs WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]bool)
	for rows.Next() {
		var scope string
		var muted bool
		if err := rows.Scan(&scope, &muted); err != nil {
			return nil, err
		}
		out[scope] = muted
	}
	return out, rows.Err()
}

// SetMute upserts a single mute preference.
func (s *Store) SetMute(ctx context.Context, userID int64, scope string, muted bool) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO mute_prefs (user_id, scope, muted) VALUES ($1,$2,$3)
		 ON CONFLICT (user_id, scope) DO UPDATE SET muted = EXCLUDED.muted`,
		userID, scope, muted)
	return err
}

// IsMuted reports whether a user has muted notifications for a given event
// category and group, considering 'all', the category, and the specific group.
func IsMuted(prefs map[string]bool, category string, groupID int64) bool {
	if prefs["all"] {
		return true
	}
	if prefs[category] {
		return true
	}
	if prefs[groupScope(groupID)] {
		return true
	}
	return false
}

func groupScope(groupID int64) string {
	return "group:" + strconv.FormatInt(groupID, 10)
}
