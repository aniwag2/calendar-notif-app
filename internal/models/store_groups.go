package models

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// CreateGroup creates a group within an org and returns it.
func (s *Store) CreateGroup(ctx context.Context, orgID int64, name string, patientID, staffID *int64) (*Group, error) {
	g := &Group{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO groups (org_id, name, patient_id, staff_id)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, org_id, name, patient_id, staff_id, created_at`,
		orgID, name, patientID, staffID,
	).Scan(&g.ID, &g.OrgID, &g.Name, &g.PatientID, &g.StaffID, &g.CreatedAt)
	if err != nil {
		return nil, err
	}
	return g, nil
}

// AddGroupMember links a user (typically family) to a group; idempotent.
func (s *Store) AddGroupMember(ctx context.Context, groupID, userID int64) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO group_members (group_id, user_id) VALUES ($1, $2)
		 ON CONFLICT DO NOTHING`, groupID, userID)
	return err
}

// GroupByID returns a single group scoped to an org.
func (s *Store) GroupByID(ctx context.Context, orgID, id int64) (*Group, error) {
	g := &Group{}
	err := s.pool.QueryRow(ctx,
		`SELECT g.id, g.org_id, g.name, g.patient_id, g.staff_id, g.created_at,
		        COALESCE(p.name, '')
		 FROM groups g
		 LEFT JOIN users p ON p.id = g.patient_id
		 WHERE g.id = $1 AND g.org_id = $2`,
		id, orgID,
	).Scan(&g.ID, &g.OrgID, &g.Name, &g.PatientID, &g.StaffID, &g.CreatedAt, &g.PatientName)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return g, err
}

// GroupsForOrg returns all groups in an org (admin/staff view).
func (s *Store) GroupsForOrg(ctx context.Context, orgID int64) ([]Group, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT g.id, g.org_id, g.name, g.patient_id, g.staff_id, g.created_at,
		        COALESCE(p.name, '')
		 FROM groups g
		 LEFT JOIN users p ON p.id = g.patient_id
		 WHERE g.org_id = $1
		 ORDER BY g.name`, orgID)
	if err != nil {
		return nil, err
	}
	return scanGroups(rows)
}

// GroupsForUser returns groups a patient/family member belongs to.
func (s *Store) GroupsForUser(ctx context.Context, userID int64) ([]Group, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT DISTINCT g.id, g.org_id, g.name, g.patient_id, g.staff_id, g.created_at,
		        COALESCE(p.name, '')
		 FROM groups g
		 LEFT JOIN users p ON p.id = g.patient_id
		 LEFT JOIN group_members gm ON gm.group_id = g.id
		 WHERE g.patient_id = $1 OR gm.user_id = $1
		 ORDER BY g.name`, userID)
	if err != nil {
		return nil, err
	}
	return scanGroups(rows)
}

// GroupMembers returns all users linked to a group (family members).
func (s *Store) GroupMembers(ctx context.Context, groupID int64) ([]User, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT u.id, u.org_id, u.name, u.email, u.password_hash, u.role, u.created_at
		 FROM users u
		 JOIN group_members gm ON gm.user_id = u.id
		 WHERE gm.group_id = $1
		 ORDER BY u.name`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.OrgID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// GroupRecipients returns the patient and family members of a group (for notifications).
func (s *Store) GroupRecipients(ctx context.Context, groupID int64) ([]User, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT u.id, u.org_id, u.name, u.email, u.password_hash, u.role, u.created_at
		 FROM users u
		 WHERE u.id = (SELECT patient_id FROM groups WHERE id = $1)
		 UNION
		 SELECT u.id, u.org_id, u.name, u.email, u.password_hash, u.role, u.created_at
		 FROM users u
		 JOIN group_members gm ON gm.user_id = u.id
		 WHERE gm.group_id = $1`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.OrgID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// DeleteGroup removes a group (events + memberships cascade).
func (s *Store) DeleteGroup(ctx context.Context, orgID, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM groups WHERE id = $1 AND org_id = $2`, id, orgID)
	return err
}

// UserInGroup reports whether a user is the patient or a member of a group.
func (s *Store) UserInGroup(ctx context.Context, userID, groupID int64) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1 FROM groups WHERE id = $2 AND patient_id = $1
		   UNION
		   SELECT 1 FROM group_members WHERE group_id = $2 AND user_id = $1
		 )`, userID, groupID).Scan(&ok)
	return ok, err
}

func scanGroups(rows pgx.Rows) ([]Group, error) {
	defer rows.Close()
	var out []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.OrgID, &g.Name, &g.PatientID, &g.StaffID, &g.CreatedAt, &g.PatientName); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
