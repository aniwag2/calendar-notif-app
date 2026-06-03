package models

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

// Store provides data access methods over a pgx pool.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// --- Organizations ---

func (s *Store) CreateOrganization(ctx context.Context, name, slug string) (*Organization, error) {
	o := &Organization{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ($1, $2)
		 RETURNING id, name, slug, billing_status, allowed_cidrs, created_at`,
		name, slug,
	).Scan(&o.ID, &o.Name, &o.Slug, &o.BillingStatus, &o.AllowedCIDRs, &o.CreatedAt)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (s *Store) OrganizationByID(ctx context.Context, id int64) (*Organization, error) {
	o := &Organization{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, slug, billing_status, allowed_cidrs, created_at
		 FROM organizations WHERE id = $1`, id,
	).Scan(&o.ID, &o.Name, &o.Slug, &o.BillingStatus, &o.AllowedCIDRs, &o.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return o, err
}

func (s *Store) OrganizationBySlug(ctx context.Context, slug string) (*Organization, error) {
	o := &Organization{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, slug, billing_status, allowed_cidrs, created_at
		 FROM organizations WHERE slug = $1`, slug,
	).Scan(&o.ID, &o.Name, &o.Slug, &o.BillingStatus, &o.AllowedCIDRs, &o.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return o, err
}

func (s *Store) CountOrganizations(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM organizations`).Scan(&n)
	return n, err
}

// --- Users ---

func (s *Store) CreateUser(ctx context.Context, orgID int64, name, email, passwordHash, role string) (*User, error) {
	u := &User{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users (org_id, name, email, password_hash, role)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, org_id, name, email, password_hash, role, created_at`,
		orgID, name, email, passwordHash, role,
	).Scan(&u.ID, &u.OrgID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Store) UserByEmail(ctx context.Context, email string) (*User, error) {
	u := &User{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, org_id, name, email, password_hash, role, created_at
		 FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.OrgID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (s *Store) UserByID(ctx context.Context, id int64) (*User, error) {
	u := &User{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, org_id, name, email, password_hash, role, created_at
		 FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.OrgID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (s *Store) EmailExists(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, email).Scan(&exists)
	return exists, err
}

// UsersByOrg returns users in an org, optionally filtered by role ("" = all).
func (s *Store) UsersByOrg(ctx context.Context, orgID int64, role string) ([]User, error) {
	var rows pgx.Rows
	var err error
	if role == "" {
		rows, err = s.pool.Query(ctx,
			`SELECT id, org_id, name, email, password_hash, role, created_at
			 FROM users WHERE org_id = $1 ORDER BY role, name`, orgID)
	} else {
		rows, err = s.pool.Query(ctx,
			`SELECT id, org_id, name, email, password_hash, role, created_at
			 FROM users WHERE org_id = $1 AND role = $2 ORDER BY name`, orgID, role)
	}
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

// DeleteUser removes a user. FK rules (CASCADE / SET NULL) keep referencing rows consistent.
func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	return err
}

func (s *Store) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET password_hash = $1 WHERE id = $2`, passwordHash, id)
	return err
}

// SetRole changes a user's role.
func (s *Store) SetRole(ctx context.Context, id int64, role string) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET role = $1 WHERE id = $2`, role, id)
	return err
}

// CountAdmins returns how many admins an org has.
func (s *Store) CountAdmins(ctx context.Context, orgID int64) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM users WHERE org_id = $1 AND role = 'admin'`, orgID).Scan(&n)
	return n, err
}

// DeleteOrganization removes an org and (via FK cascade) all its users and data.
func (s *Store) DeleteOrganization(ctx context.Context, orgID int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	return err
}
