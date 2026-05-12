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

	query := model.DB.Model(&model.Order{})
	if isTeacher, teacherID := currentTeacherScope(c); isTeacher {
		subQuery := model.DB.Table("order_items oi").
			Select("DISTINCT oi.order_id").
			Joins("JOIN courses c ON c.id = oi.goods_id").
			Joins("JOIN orders o ON o.id = oi.order_id").
			Where("o.type = ?", "course").
			Where("c.teacher_id = ?", teacherID)
		query = query.Where("id IN (?)", subQuery)
	}

	var orderRows []model.Order
	if err := query.Order("created_at desc").Find(&orderRows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取订单失败"})
		return
	}

	if len(orderRows) == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": []adminOrderListItem{}})
		return
	}

	orderIDs := make([]uint64, 0, len(orderRows))
	userIDs := make([]uint64, 0, len(orderRows))
	for _, order := range orderRows {
		orderIDs = append(orderIDs, order.ID)
		userIDs = append(userIDs, order.UserID)
	}

	var users []model.User
	userMap := make(map[uint64]model.User)
	if err := model.DB.Where("id IN ?", userIDs).Find(&users).Error; err == nil {
		for _, user := range users {
			userMap[user.ID] = user
		}
	}

	var orderItems []model.OrderItem
	itemMap := make(map[uint64][]model.OrderItem)
	if err := model.DB.Where("order_id IN ?", orderIDs).Order("id asc").Find(&orderItems).Error; err == nil {
		for _, item := range orderItems {
			itemMap[item.OrderID] = append(itemMap[item.OrderID], item)
		}
	}

	orders := make([]adminOrderListItem, 0, len(orderRows))
	for _, order := range orderRows {
		user := userMap[order.UserID]
		goodsName := ""
		if items, ok := itemMap[order.ID]; ok && len(items) > 0 {
			goodsName = items[0].GoodsName
		}
		orders = append(orders, adminOrderListItem{
			ID:         order.ID,
			OrderNo:    order.OrderNo,
			User:       user.Nickname,
			Phone:      user.Phone,
			GoodsName:  goodsName,
			Amount:     order.PayAmount,
			Type:       order.Type,
			Status:     order.Status,
			CreateTime: order.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": orders,
	})
}
