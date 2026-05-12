package model

import "time"

const (
	AdminRoleSuper  = "super_admin"
	AdminRoleNormal = "admin"
)

// Admin 后台管理员账号
type Admin struct {
	ID           uint64     `gorm:"primaryKey;column:id" json:"id"`
	Username     string     `gorm:"column:username" json:"username"`
	PasswordHash string     `gorm:"column:password_hash" json:"-"`
	Name         string     `gorm:"column:name" json:"name"`
	Mobile       string     `gorm:"column:mobile" json:"mobile"`
	RoleCode     string     `gorm:"column:role_code" json:"role_code"`
	Status       int8       `gorm:"column:status" json:"status"`
	LastLoginAt  *time.Time `gorm:"column:last_login_at" json:"last_login_at"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (Admin) TableName() string {
	return "admins"
}
