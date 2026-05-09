package model

import (
	"time"
)

// LiveRoom 直播间动态表
type LiveRoom struct {
	ID              uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	LessonID        uint64     `gorm:"index;not null" json:"lesson_id"` // 关联课时ID
	TeacherID       uint64     `gorm:"index;not null" json:"teacher_id"`
	Status          int8       `gorm:"type:tinyint;default:0;not null" json:"status"` // 0=未开播, 1=直播中, 2=已结束
	ActualStartTime *time.Time `json:"actual_start_time"`
	ActualEndTime   *time.Time `json:"actual_end_time"`
	OnlineCount     int        `gorm:"default:0" json:"online_count"`
	CurrentSheetID  *uint64    `json:"current_sheet_id"` // 当前正在推送的曲谱ID
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// UserWallet 用户钱包表
type UserWallet struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      uint64    `gorm:"uniqueIndex;not null" json:"user_id"`
	CoinBalance int       `gorm:"default:0;not null" json:"coin_balance"` // 虚拟币余额
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// LiveGiftRecord 直播打赏流水表
type LiveGiftRecord struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     uint64    `gorm:"index;not null" json:"user_id"`
	LiveRoomID uint64    `gorm:"index;not null" json:"live_room_id"`
	GiftID     int       `gorm:"not null" json:"gift_id"`
	Amount     int       `gorm:"not null" json:"amount"` // 扣除的虚拟币数量
	CreatedAt  time.Time `json:"created_at"`
}
