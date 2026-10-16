package model

import "time"

// Delivery settings are separate from legacy course schemas.
type CourseDeliverySetting struct {
	CourseID     uint64 `gorm:"primaryKey" json:"course_id"`
	ValidityDays int    `json:"validity_days"`
	Sequential   bool   `json:"sequential"`
}

type CourseCohort struct {
	ID                 uint64    `gorm:"primaryKey" json:"id"`
	CourseID           uint64    `json:"course_id"`
	Name               string    `json:"name"`
	Capacity           int       `json:"capacity"`
	Price              int       `json:"price"` // cents
	EnrollmentDeadline time.Time `json:"enrollment_deadline"`
	StartAt            time.Time `json:"start_at"`
	EndAt              time.Time `json:"end_at"`
	Status             string    `json:"status"`
	CreatedAt          time.Time `json:"created_at"`
}

type CourseEnrollment struct {
	ID        uint64     `gorm:"primaryKey" json:"id"`
	CourseID  uint64     `gorm:"uniqueIndex:uq_enrollment" json:"course_id"`
	CohortID  uint64     `gorm:"uniqueIndex:uq_enrollment" json:"cohort_id"`
	UserID    uint64     `gorm:"uniqueIndex:uq_enrollment" json:"user_id"`
	OrderID   uint64     `json:"order_id"`
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
}
