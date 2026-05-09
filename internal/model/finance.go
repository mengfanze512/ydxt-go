package model

import "time"

// FinanceSettlement 讲师结算单表
type FinanceSettlement struct {
	ID            uint64    `gorm:"primaryKey" json:"id"`
	TeacherID     uint64    `gorm:"column:teacher_id;index" json:"teacher_id"`
	Month         string    `gorm:"column:month;type:varchar(32)" json:"month"` // e.g., 2026-05
	BaseSalary    int       `gorm:"column:base_salary;type:int unsigned" json:"base_salary"`
	CourseRevenue int       `gorm:"column:course_revenue;type:int unsigned" json:"course_revenue"`
	GiftRevenue   int       `gorm:"column:gift_revenue;type:int unsigned" json:"gift_revenue"`
	TotalAmount   int       `gorm:"column:total_amount;type:int unsigned" json:"total_amount"`
	Status        string    `gorm:"column:status;type:varchar(32);default:'pending'" json:"status"` // pending/settled
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (FinanceSettlement) TableName() string {
	return "finance_settlements"
}
