package model

import "time"

// Order 主订单表
type Order struct {
	ID          uint64    `gorm:"primaryKey" json:"id"`
	OrderNo     string    `gorm:"column:order_no;type:varchar(64);uniqueIndex" json:"order_no"`
	UserID      uint64    `gorm:"column:user_id;index" json:"user_id"`
	Type        string    `gorm:"column:type;type:varchar(32)" json:"type"` // course, shop
	Amount      int       `gorm:"column:amount;type:int unsigned" json:"amount"`
	PayAmount   int       `gorm:"column:pay_amount;type:int unsigned" json:"pay_amount"`
	Status      string    `gorm:"column:status;type:varchar(32);default:'unpaid'" json:"status"` // unpaid, paid, shipped, refunded
	PayMethod   int8      `gorm:"column:pay_method;type:tinyint" json:"pay_method"`              // 1=WeChat, 2=Alipay
	PayTime     time.Time `gorm:"column:pay_time" json:"pay_time"`
	ThirdTrxID  string    `gorm:"column:third_trx_id;type:varchar(128)" json:"third_trx_id"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
}

func (Order) TableName() string {
	return "orders"
}

// OrderItem 订单明细表
type OrderItem struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	OrderID   uint64    `gorm:"column:order_id;index" json:"order_id"`
	GoodsID   uint64    `gorm:"column:goods_id;index" json:"goods_id"`
	GoodsName string    `gorm:"column:goods_name;type:varchar(128)" json:"goods_name"`
	Price     int       `gorm:"column:price;type:int unsigned" json:"price"`
	Quantity  int       `gorm:"column:quantity;type:int" json:"quantity"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (OrderItem) TableName() string {
	return "order_items"
}
