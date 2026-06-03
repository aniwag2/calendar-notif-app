package models

import "time"

type Organization struct {
	ID            int64
	Name          string
	Slug          string
	BillingStatus string
	AllowedCIDRs  string
	CreatedAt     time.Time
}

type User struct {
	ID           int64
	OrgID        int64
	Name         string
	Email        string
	PasswordHash string
	Role         string // admin | staff | patient | family
	CreatedAt    time.Time
}

func (u User) IsStaffSide() bool { return u.Role == "admin" || u.Role == "staff" }

type Group struct {
	ID          int64
	OrgID       int64
	Name        string
	PatientID   *int64
	StaffID     *int64
	PatientName string // joined, optional
	CreatedAt   time.Time
}

type Event struct {
	ID          int64
	OrgID       int64
	GroupID     int64
	Category    string // therapy | activity | appointment
	Title       string
	Description string
	StartsAt    time.Time
	EndsAt      *time.Time
	Recurrence  string // none | weekly
	CreatedBy   *int64
	CreatedAt   time.Time
}

type Notification struct {
	ID        int64
	OrgID     int64
	UserID    int64
	EventID   *int64
	Message   string
	ReadAt    *time.Time
	CreatedAt time.Time
}
