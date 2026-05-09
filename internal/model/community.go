package model

import "time"

// CommunityPost 社区帖子表
type CommunityPost struct {
	ID            uint64    `gorm:"primaryKey" json:"id"`
	UserID        uint64    `gorm:"column:user_id;index" json:"user_id"`
	Content       string    `gorm:"column:content;type:text" json:"content"`
	Images        string    `gorm:"column:images;type:json" json:"images"`
	LikesCount    int       `gorm:"column:likes_count;type:int;default:0" json:"likes_count"`
	CommentsCount int       `gorm:"column:comments_count;type:int;default:0" json:"comments_count"`
	Status        string    `gorm:"column:status;type:varchar(32);default:'normal'" json:"status"` // normal/pending/deleted
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
}

func (CommunityPost) TableName() string {
	return "community_posts"
}
