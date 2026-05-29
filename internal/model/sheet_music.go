package model

import "time"

// SheetMusic 曲谱表
type SheetMusic struct {
	ID         uint64    `gorm:"primaryKey" json:"id"`
	Title      string    `gorm:"column:title;type:varchar(128)" json:"title"`
	Author     string    `gorm:"column:author;type:varchar(64)" json:"author"`
	Instrument string    `gorm:"column:instrument;type:varchar(32)" json:"instrument"`
	Difficulty int8      `gorm:"column:difficulty;type:tinyint" json:"difficulty"`
	IsFree     int8      `gorm:"column:is_free;type:tinyint;default:1" json:"is_free"` // 1=免费, 0=收费
	Price      int       `gorm:"column:price;type:int unsigned;default:0" json:"price"`
	CoverURL   string    `gorm:"column:cover_url;type:varchar(255)" json:"cover_url"`
	ContentURL string    `gorm:"column:content_url;type:text" json:"content_url"` // 可存 JSON 数组
	Downloads  int       `gorm:"column:downloads;type:int;default:0" json:"downloads"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (SheetMusic) TableName() string {
	return "sheet_musics"
}
