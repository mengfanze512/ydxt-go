package model

import "time"

// CourseChapter 课程章节
type CourseChapter struct {
	ID        uint64    `gorm:"primaryKey;column:id" json:"id"`
	CourseID  uint64    `gorm:"column:course_id;index" json:"course_id"`
	Title     string    `gorm:"column:title" json:"title"`
	SortOrder int       `gorm:"column:sort_order" json:"sort_order"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (CourseChapter) TableName() string {
	return "course_chapters"
}

// CourseLesson 课程小节
type CourseLesson struct {
	ID            uint64     `gorm:"primaryKey;column:id" json:"id"`
	ChapterID     uint64     `gorm:"column:chapter_id;index" json:"chapter_id"`
	Title         string     `gorm:"column:title" json:"title"`
	Type          int8       `gorm:"column:type" json:"type"` // 1=视频, 2=图文, 3=直播
	MediaURL      string     `gorm:"column:media_url" json:"media_url"`
	DetailContent string     `gorm:"column:detail_content" json:"detail_content"`
	LiveRoomID    string     `gorm:"column:live_room_id" json:"live_room_id"`
	LiveStart     *time.Time `gorm:"column:live_start" json:"live_start"`
	ExpireAt      *time.Time `gorm:"column:expire_at" json:"expire_at"`
	Duration      int        `gorm:"column:duration" json:"duration"`
	IsFree        int8       `gorm:"column:is_free" json:"is_free"`
	SortOrder     int        `gorm:"column:sort_order" json:"sort_order"`
	CreatedAt     time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (CourseLesson) TableName() string {
	return "course_lessons"
}
