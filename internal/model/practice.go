package model

import "time"

// UserCheckin 用户打卡表
type UserCheckin struct {
	ID             uint64    `gorm:"primaryKey" json:"id"`
	UserID         uint64    `gorm:"column:user_id;index" json:"user_id"`
	CheckinDate    string    `gorm:"column:checkin_date;type:date" json:"checkin_date"`
	ContinuousDays int       `gorm:"column:continuous_days;type:int;default:1" json:"continuous_days"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at"`
}

func (UserCheckin) TableName() string {
	return "user_checkins"
}

// PracticeRecord 学员AI陪练记录表
type PracticeRecord struct {
	ID           uint64    `gorm:"primaryKey" json:"id"`
	UserID       uint64    `gorm:"column:user_id;index" json:"user_id"`
	SheetMusicID uint64    `gorm:"column:sheet_music_id;index" json:"sheet_music_id"`
	AudioURL     string    `gorm:"column:audio_url;type:varchar(255)" json:"audio_url"`
	TotalScore   int       `gorm:"column:total_score;type:int" json:"total_score"`
	Dimensions   string    `gorm:"column:dimensions;type:json" json:"dimensions"`     // 详细多维度得分 JSON
	AIComment    string    `gorm:"column:ai_comment;type:text" json:"ai_comment"`     // AI文字点评
	MistakeBars  string    `gorm:"column:mistake_bars;type:json" json:"mistake_bars"` // 出错小节 JSON
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
}

func (PracticeRecord) TableName() string {
	return "practice_records"
}
