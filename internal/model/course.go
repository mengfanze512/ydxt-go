package model

import "time"

// Course 对应表 courses 的映射实体模型
type Course struct {
	ID           uint64    `gorm:"primaryKey;column:id;autoIncrement" json:"id"`
	TeacherID    uint64    `gorm:"column:teacher_id" json:"teacher_id"`     // 讲师ID
	Title        string    `gorm:"column:title" json:"title"`               // 课程标题
	Cover        string    `gorm:"column:cover_url" json:"cover"`           // 课程封面图
	Category     int       `gorm:"column:category" json:"category"`         // 乐器分类ID
	Difficulty   int8      `gorm:"column:difficulty" json:"difficulty"`     // 难度: 1=入门, 2=进阶, 3=考级
	Type         int8      `gorm:"column:type" json:"type"`                 // 类型: 1=录播, 2=直播, 3=训练营
	Price        int       `gorm:"column:price" json:"price"`               // 现价 (单位：分)
	OriginalPrice int      `gorm:"column:original_price" json:"original_price"` // 原价 (单位：分)
	SalesCount   int       `gorm:"column:sales_count" json:"sales_count"`   // 真实销量
	FakeSales    int       `gorm:"column:fake_sales" json:"fake_sales"`     // 虚拟销量基数
	Status       int8      `gorm:"column:status" json:"status"`             // 状态: 0=草稿, 1=已上架, 2=已下架
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at"`
	IsDeleted    int8      `gorm:"column:is_deleted" json:"is_deleted"`

	// 以下为前端兼容或关联查询用的虚拟字段，不映射到数据库
	TeacherName  string    `gorm:"-" json:"teacher_name"`
	Level        string    `gorm:"-" json:"level"`
	Desc         string    `gorm:"-" json:"description"`
	StudentCount int       `gorm:"-" json:"student_count"`
}

// TableName 指定自定义表名
func (Course) TableName() string {
	return "courses"
}
