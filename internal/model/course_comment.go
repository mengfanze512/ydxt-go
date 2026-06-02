package model

import "time"

const (
	CourseLessonCommentStatusApproved = "approved"
	CourseLessonCommentStatusDeleted  = "deleted"
)

// CourseLessonComment 课程小节评论表
type CourseLessonComment struct {
	ID            uint64    `gorm:"primaryKey" json:"id"`
	CourseID      uint64    `gorm:"column:course_id;index:idx_course_lesson_status,priority:1;index" json:"course_id"`
	LessonID      uint64    `gorm:"column:lesson_id;index:idx_course_lesson_status,priority:2;index" json:"lesson_id"`
	UserID        uint64    `gorm:"column:user_id;index" json:"user_id"`
	ParentID      uint64    `gorm:"column:parent_id;index" json:"parent_id"`
	ReplyToUserID uint64    `gorm:"column:reply_to_user_id;index" json:"reply_to_user_id"`
	Content       string    `gorm:"column:content;type:text" json:"content"`
	LikesCount    int       `gorm:"column:likes_count;type:int;default:0" json:"likes_count"`
	Status        string    `gorm:"column:status;type:varchar(32);default:'approved';index:idx_course_lesson_status,priority:3" json:"status"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (CourseLessonComment) TableName() string {
	return "course_lesson_comments"
}

// CourseLessonCommentLike 课程小节评论点赞表
type CourseLessonCommentLike struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	CommentID uint64    `gorm:"column:comment_id;index;uniqueIndex:uniq_course_lesson_comment_user" json:"comment_id"`
	UserID    uint64    `gorm:"column:user_id;index;uniqueIndex:uniq_course_lesson_comment_user" json:"user_id"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (CourseLessonCommentLike) TableName() string {
	return "course_lesson_comment_likes"
}
