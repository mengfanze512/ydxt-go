package model

import "time"

// ShopGoods 商城商品表
type ShopGoods struct {
	ID            uint64    `gorm:"primaryKey" json:"id"`
	Title         string    `gorm:"column:title;type:varchar(128)" json:"title"`
	Description   string    `gorm:"column:description;type:text" json:"description"`
	Price         int       `gorm:"column:price;type:int unsigned" json:"price"`                   // 现价(分)
	OriginalPrice int       `gorm:"column:original_price;type:int unsigned" json:"original_price"` // 原价(分)
	Stock         int       `gorm:"column:stock;type:int;default:0" json:"stock"`
	Sales         int       `gorm:"column:sales;type:int;default:0" json:"sales"`
	CoverURL      string    `gorm:"column:cover_url;type:varchar(255)" json:"cover_url"`
	Category      string    `gorm:"column:category;type:varchar(32)" json:"category"`
	Status        int8      `gorm:"column:status;type:tinyint;default:1" json:"status"` // 1=上架, 0=下架
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (ShopGoods) TableName() string {
	return "shop_goods"
}

// CartItem 购物车明细表
type CartItem struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	UserID    uint64    `gorm:"column:user_id;index" json:"user_id"`
	GoodsID   uint64    `gorm:"column:goods_id;index" json:"goods_id"`
	Spec      string    `gorm:"column:spec;type:varchar(64)" json:"spec"`
	Quantity  int       `gorm:"column:quantity;type:int;default:1" json:"quantity"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (CartItem) TableName() string {
	return "cart_items"
}
