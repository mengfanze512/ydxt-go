package api

import (
	"fmt"
	"net/http"
	"time"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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

type createShopOrderRequest struct {
	CartItemIDs []uint64 `json:"cart_item_ids"`
}

type userOrderItem struct {
	GoodsID   uint64  `json:"goods_id"`
	GoodsName string  `json:"goods_name"`
	Image     string  `json:"image"`
	Spec      string  `json:"spec"`
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
}

type userOrderListItem struct {
	ID         uint64          `json:"id"`
	OrderNo    string          `json:"order_no"`
	Type       string          `json:"type"`
	Status     string          `json:"status"`
	Amount     float64         `json:"amount"`
	PayAmount  float64         `json:"pay_amount"`
	CreateTime string          `json:"create_time"`
	Items      []userOrderItem `json:"items"`
}

type createShopOrderResponse struct {
	ID         uint64  `json:"id"`
	OrderNo    string  `json:"order_no"`
	Status     string  `json:"status"`
	Amount     float64 `json:"amount"`
	PayAmount  float64 `json:"pay_amount"`
	CreateTime string  `json:"create_time"`
}

func generateOrderNo() string {
	return fmt.Sprintf("YD%s%04d", time.Now().Format("20060102150405"), time.Now().Nanosecond()%10000)
}

func currentOrderUserID(c *gin.Context) (uint64, bool) {
	userID, ok := c.Get("userID")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录"})
		return 0, false
	}
	id, ok := userID.(uint64)
	if !ok || id == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "用户信息无效"})
		return 0, false
	}
	return id, true
}

// CreateShopOrder 从购物车创建商城订单（先打通真实下单，支付后续再接入）
func CreateShopOrder(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "数据库未连接"})
		return
	}

	userID, ok := currentOrderUserID(c)
	if !ok {
		return
	}

	var req createShopOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.CartItemIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请选择要下单的购物车商品"})
		return
	}

	uniqueIDs := make([]uint64, 0, len(req.CartItemIDs))
	seen := make(map[uint64]struct{})
	for _, id := range req.CartItemIDs {
		if id == 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}
	if len(uniqueIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请选择要下单的购物车商品"})
		return
	}

	var cartItems []model.CartItem
	if err := model.DB.Where("user_id = ? AND item_type = ? AND id IN ?", userID, cartItemTypeShop, uniqueIDs).Find(&cartItems).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "读取购物车失败"})
		return
	}
	if len(cartItems) != len(uniqueIDs) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "部分购物车商品不存在或已失效"})
		return
	}

	goodsIDs := make([]uint64, 0, len(cartItems))
	for _, item := range cartItems {
		goodsIDs = append(goodsIDs, item.GoodsID)
	}

	var goodsList []model.ShopGoods
	if err := model.DB.Where("id IN ? AND status = ?", goodsIDs, 1).Find(&goodsList).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "读取商品信息失败"})
		return
	}
	goodsMap := make(map[uint64]model.ShopGoods, len(goodsList))
	for _, goods := range goodsList {
		goodsMap[goods.ID] = goods
	}

	totalAmount := 0
	orderItems := make([]model.OrderItem, 0, len(cartItems))
	for _, item := range cartItems {
		goods, exists := goodsMap[item.GoodsID]
		if !exists {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "部分商品已下架或不存在"})
			return
		}
		if goods.Stock > 0 && item.Quantity > goods.Stock {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": goods.Title + " 库存不足"})
			return
		}

		subtotal := goods.Price * item.Quantity
		totalAmount += subtotal
		orderItems = append(orderItems, model.OrderItem{
			GoodsID:   goods.ID,
			GoodsName: goods.Title,
			Price:     goods.Price,
			Quantity:  item.Quantity,
		})
	}

	order := model.Order{
		OrderNo:   generateOrderNo(),
		UserID:    userID,
		Type:      "shop",
		Amount:    totalAmount,
		PayAmount: totalAmount,
		Status:    "unpaid",
		PayMethod: 1,
	}

	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		for i := range orderItems {
			orderItems[i].OrderID = order.ID
		}
		if err := tx.Create(&orderItems).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ? AND item_type = ? AND id IN ?", userID, cartItemTypeShop, uniqueIDs).Delete(&model.CartItem{}).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "创建订单失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": createShopOrderResponse{
			ID:         order.ID,
			OrderNo:    order.OrderNo,
			Status:     order.Status,
			Amount:     formatGoodsPrice(order.Amount),
			PayAmount:  formatGoodsPrice(order.PayAmount),
			CreateTime: order.CreatedAt.Format("2006-01-02 15:04:05"),
		},
	})
}

// GetMyOrders 获取当前用户订单列表
func GetMyOrders(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": []userOrderListItem{}})
		return
	}

	userID, ok := currentOrderUserID(c)
	if !ok {
		return
	}

	query := model.DB.Where("user_id = ? AND type = ?", userID, "shop")
	status := c.Query("status")
	if status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}

	var orders []model.Order
	if err := query.Order("created_at desc").Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取订单失败"})
		return
	}
	if len(orders) == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": []userOrderListItem{}})
		return
	}

	orderIDs := make([]uint64, 0, len(orders))
	for _, order := range orders {
		orderIDs = append(orderIDs, order.ID)
	}

	var orderItems []model.OrderItem
	if err := model.DB.Where("order_id IN ?", orderIDs).Order("id asc").Find(&orderItems).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取订单失败"})
		return
	}

	itemMap := make(map[uint64][]model.OrderItem)
	goodsIDs := make([]uint64, 0, len(orderItems))
	goodsSeen := make(map[uint64]struct{})
	for _, item := range orderItems {
		itemMap[item.OrderID] = append(itemMap[item.OrderID], item)
		if _, exists := goodsSeen[item.GoodsID]; !exists {
			goodsSeen[item.GoodsID] = struct{}{}
			goodsIDs = append(goodsIDs, item.GoodsID)
		}
	}

	goodsImageMap := make(map[uint64]string)
	if len(goodsIDs) > 0 {
		var goodsList []model.ShopGoods
		if err := model.DB.Where("id IN ?", goodsIDs).Find(&goodsList).Error; err == nil {
			for _, goods := range goodsList {
				goodsImageMap[goods.ID] = goods.CoverURL
			}
		}
	}

	result := make([]userOrderListItem, 0, len(orders))
	for _, order := range orders {
		items := itemMap[order.ID]
		respItems := make([]userOrderItem, 0, len(items))
		for _, item := range items {
			respItems = append(respItems, userOrderItem{
				GoodsID:   item.GoodsID,
				GoodsName: item.GoodsName,
				Image:     goodsImageMap[item.GoodsID],
				Price:     formatGoodsPrice(item.Price),
				Quantity:  item.Quantity,
			})
		}

		result = append(result, userOrderListItem{
			ID:         order.ID,
			OrderNo:    order.OrderNo,
			Type:       order.Type,
			Status:     order.Status,
			Amount:     formatGoodsPrice(order.Amount),
			PayAmount:  formatGoodsPrice(order.PayAmount),
			CreateTime: order.CreatedAt.Format("2006-01-02 15:04:05"),
			Items:      respItems,
		})
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": result})
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
