package model

import "time"

// User 对应表 users 的映射实体模型
type User struct {
	ID        uint64    `gorm:"primaryKey;column:id"`
	Phone     string    `gorm:"column:phone"`
	Password  string    `gorm:"column:password"` // 密码，用于后台登录
	OpenID    string    `gorm:"column:openid"`
	UnionID   string    `gorm:"column:unionid"`
	Nickname  string    `gorm:"column:nickname"`
	Avatar    string    `gorm:"column:avatar"`
	Role      int8      `gorm:"column:role"`   // 1=学生, 2=讲师, 9=管理员
	Status    int8      `gorm:"column:status"` // 1=正常, 0=禁用
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
	IsDeleted int8      `gorm:"column:is_deleted"`
}

// TableName 指定自定义表名
func (User) TableName() string {
	return "users"
}
