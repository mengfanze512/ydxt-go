package api

import (
	"net/http"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
)

type adminOrderListItem struct {
	ID         uint64 `json:"id"`
	OrderNo    string `json:"order_no"`
	User       string `json:"user"`
	Phone      string `json:"phone"`
	GoodsName  string `json:"goods_name"`
	Amount     int    `json:"amount"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	CreateTime string `json:"create_time"`
}

// GetOrderList 获取订单列表
func GetOrderList(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": []adminOrderListItem{}})
		return
	}

	var orders []adminOrderListItem
	query := model.DB.Table("orders o").
		Select(`
			o.id,
			o.order_no,
			COALESCE(u.nickname, '') as user,
			COALESCE(u.phone, '') as phone,
			COALESCE(MAX(oi.goods_name), '') as goods_name,
			o.pay_amount as amount,
			o.type,
			o.status,
			DATE_FORMAT(o.created_at, '%Y-%m-%d %H:%i:%s') as create_time
		`).
		Joins("LEFT JOIN users u ON u.id = o.user_id").
		Joins("LEFT JOIN order_items oi ON oi.order_id = o.id")
	if isTeacher, teacherID := currentTeacherScope(c); isTeacher {
		query = query.Joins("JOIN courses c ON c.id = oi.goods_id AND o.type = 'course'").
			Where("c.teacher_id = ?", teacherID)
	}
	if err := query.
		Group("o.id, o.order_no, u.nickname, u.phone, o.pay_amount, o.type, o.status, o.created_at").
		Order("o.created_at desc").
		Scan(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取订单失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": orders,
	})
}
