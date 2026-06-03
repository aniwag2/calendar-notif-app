package models

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// CreateEvent inserts an event and returns it.
func (s *Store) CreateEvent(ctx context.Context, e *Event) (*Event, error) {
	err := s.pool.QueryRow(ctx,
		`INSERT INTO events (org_id, group_id, category, title, description, starts_at, ends_at, recurrence, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id, created_at`,
		e.OrgID, e.GroupID, e.Category, e.Title, e.Description, e.StartsAt, e.EndsAt, e.Recurrence, e.CreatedBy,
	).Scan(&e.ID, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	return e, nil
}

// EventsForCalendar returns events relevant to a [from,to] window for a group:
// non-recurring events that start within the window, plus all weekly-recurring
// events whose first occurrence is on or before the window end (expanded in code).
func (s *Store) EventsForCalendar(ctx context.Context, groupID int64, from, to time.Time) ([]Event, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, org_id, group_id, category, title, description, starts_at, ends_at, recurrence, created_by, created_at
		 FROM events
		 WHERE group_id = $1
		   AND (
		     (recurrence = 'none' AND starts_at >= $2 AND starts_at < $3)
		     OR (recurrence = 'weekly' AND starts_at < $3)
		   )
		 ORDER BY starts_at`, groupID, from, to)
	if err != nil {
		return nil, err
	}
	return scanEvents(rows)
}

// RecentEvents returns the most recent events for a group (activity feed).
func (s *Store) RecentEvents(ctx context.Context, groupID int64, limit int) ([]Event, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, org_id, group_id, category, title, description, starts_at, ends_at, recurrence, created_by, created_at
		 FROM events WHERE group_id = $1 ORDER BY created_at DESC LIMIT $2`, groupID, limit)
	if err != nil {
		return nil, err
	}
	return scanEvents(rows)
}

// DeleteEvent removes an event scoped to an org.
func (s *Store) DeleteEvent(ctx context.Context, orgID, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM events WHERE id = $1 AND org_id = $2`, id, orgID)
	return err
}

func scanEvents(rows pgx.Rows) ([]Event, error) {
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.OrgID, &e.GroupID, &e.Category, &e.Title, &e.Description,
			&e.StartsAt, &e.EndsAt, &e.Recurrence, &e.CreatedBy, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
