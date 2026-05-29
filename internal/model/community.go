package model

import "time"

const (
	CommunityPostStatusPending  = "pending"
	CommunityPostStatusApproved = "approved"
	CommunityPostStatusRejected = "rejected"
	CommunityPostStatusDeleted  = "deleted"
)

// CommunityPost 社区帖子表
type CommunityPost struct {
	ID             uint64    `gorm:"primaryKey" json:"id"`
	UserID         uint64    `gorm:"column:user_id;index" json:"user_id"`
	Title          string    `gorm:"column:title;type:varchar(200)" json:"title"`
	Content        string    `gorm:"column:content;type:text" json:"content"`
	Images         string    `gorm:"column:images;type:json" json:"images"`
	VideoURL       string    `gorm:"column:video_url;type:varchar(1024)" json:"video_url"`
	VideoCoverURL  string    `gorm:"column:video_cover_url;type:varchar(1024)" json:"video_cover_url"`
	LikesCount     int       `gorm:"column:likes_count;type:int;default:0" json:"likes_count"`
	FavoritesCount int       `gorm:"column:favorites_count;type:int;default:0" json:"favorites_count"`
	CommentsCount  int       `gorm:"column:comments_count;type:int;default:0" json:"comments_count"`
	IsPinned       int8      `gorm:"column:is_pinned;type:tinyint;default:0" json:"is_pinned"`
	IsFeatured     int8      `gorm:"column:is_featured;type:tinyint;default:0" json:"is_featured"`
	Status         string    `gorm:"column:status;type:varchar(32);default:'pending';index" json:"status"` // pending/approved/rejected/deleted
	RejectReason   string    `gorm:"column:reject_reason;type:varchar(255)" json:"reject_reason"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (CommunityPost) TableName() string {
	return "community_posts"
}

type CommunityPostLike struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	PostID    uint64    `gorm:"column:post_id;index;uniqueIndex:uniq_post_user" json:"post_id"`
	UserID    uint64    `gorm:"column:user_id;index;uniqueIndex:uniq_post_user" json:"user_id"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (CommunityPostLike) TableName() string {
	return "community_post_likes"
}

type CommunityPostFavorite struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	PostID    uint64    `gorm:"column:post_id;index;uniqueIndex:uniq_post_user" json:"post_id"`
	UserID    uint64    `gorm:"column:user_id;index;uniqueIndex:uniq_post_user" json:"user_id"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (CommunityPostFavorite) TableName() string {
	return "community_post_favorites"
}
