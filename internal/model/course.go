package model

import "time"

// Course 对应表 courses 的映射实体模型
type Course struct {
	ID             uint64    `gorm:"primaryKey;column:id" json:"id"`
	Title          string    `gorm:"column:title" json:"title"`                     // 课程标题
	Cover          string    `gorm:"column:cover" json:"-"`                         // 课程封面图(旧字段，不再对外返回)
	CoverURL       string    `gorm:"column:cover_url" json:"cover_url"`             // 课程封面URL
	Price          float64   `gorm:"column:price;type:decimal(10,2)" json:"price"`  // 价格
	TeacherID      uint64    `gorm:"column:teacher_id" json:"teacher_id"`           // 讲师ID
	TeacherName    string    `gorm:"-" json:"teacher_name"`                         // 讲师名称(关联查询用)
	Category       string    `gorm:"column:category" json:"category"`               // 分类：如 "入门", "进阶", "考级"
	Level          string    `gorm:"column:level" json:"level"`                     // 课程类型："system"系统课, "1v1"私教, "vip"会员课
	Desc           string    `gorm:"column:description" json:"description"`         // 课程描述
	DetailContent  string    `gorm:"column:detail_content" json:"detail_content"`   // 课程详情(富文本HTML)
	CarouselImages string    `gorm:"column:carousel_images" json:"carousel_images"` // 轮播图(JSON数组字符串)
	StudentCount   int       `gorm:"column:student_count" json:"student_count"`     // 学习人数
	IsFeatured     int8      `gorm:"column:is_featured" json:"is_featured"`         // 1=精选课程, 0=普通课程
	Status         int8      `gorm:"column:status" json:"status"`                   // 1=上架, 0=下架
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 指定自定义表名
func (Course) TableName() string {
	return "courses"
}
