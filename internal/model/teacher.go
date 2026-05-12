package model

import "time"

// Teacher 讲师账号与资料
type Teacher struct {
	ID           uint64     `gorm:"primaryKey;column:id" json:"id"`
	Username     string     `gorm:"column:username" json:"username"`
	PasswordHash string     `gorm:"column:password_hash" json:"-"`
	Name         string     `gorm:"column:name" json:"name"`
	Mobile       string     `gorm:"column:mobile" json:"mobile"`
	Avatar       string     `gorm:"column:avatar" json:"avatar"`
	Bio          string     `gorm:"column:bio" json:"bio"`
	Intro        string     `gorm:"column:intro" json:"intro"`
	Specialty    string     `gorm:"column:specialty" json:"specialty"`
	Status       int8       `gorm:"column:status" json:"status"`
	LastLoginAt  *time.Time `gorm:"column:last_login_at" json:"last_login_at"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (Teacher) TableName() string {
	return "teachers"
}
