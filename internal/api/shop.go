package api

import (
	"net/http"
	"strconv"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
)

type shopGoodsResponse struct {
	ID            uint64  `json:"id"`
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	Price         float64 `json:"price"`
	OriginalPrice float64 `json:"original_price"`
	Stock         int     `json:"stock"`
	Sales         int     `json:"sales"`
	CoverURL      string  `json:"cover_url"`
	Category      string  `json:"category"`
	Status        int8    `json:"status"`
}

type cartItemResponse struct {
	ID        uint64  `json:"id"`
	GoodsID   uint64  `json:"goods_id"`
	Title     string  `json:"title"`
	Spec      string  `json:"spec"`
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
	Image     string  `json:"image"`
	Stock     int     `json:"stock"`
	Category  string  `json:"category"`
}

func formatGoodsPrice(price int) float64 {
	return float64(price) / 100
}

func toShopGoodsResponse(goods model.ShopGoods) shopGoodsResponse {
	return shopGoodsResponse{
		ID:            goods.ID,
		Title:         goods.Title,
		Description:   goods.Description,
		Price:         formatGoodsPrice(goods.Price),
		OriginalPrice: formatGoodsPrice(goods.OriginalPrice),
		Stock:         goods.Stock,
		Sales:         goods.Sales,
		CoverURL:      goods.CoverURL,
		Category:      goods.Category,
		Status:        goods.Status,
	}
}

// GetShopGoods 获取商城商品列表
func GetShopGoods(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": []shopGoodsResponse{}})
		return
	}

	var goods []model.ShopGoods
	query := model.DB.Where("status = ?", 1).Order("created_at desc")
	if category := c.Query("category"); category != "" && category != "全部" {
		query = query.Where("category = ?", category)
	}
	if err := query.Find(&goods).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取商品失败"})
		return
	}

	result := make([]shopGoodsResponse, 0, len(goods))
	for _, item := range goods {
		result = append(result, toShopGoodsResponse(item))
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": result})
}

// GetShopGoodsDetail 获取商品详情
func GetShopGoodsDetail(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": nil})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "商品ID无效"})
		return
	}

	var goods model.ShopGoods
	if err := model.DB.Where("id = ? AND status = ?", id, 1).First(&goods).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "商品不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": toShopGoodsResponse(goods)})
}

// GetCartItems 获取用户的购物车列表
func GetCartItems(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": []cartItemResponse{}})
		return
	}

	userID, ok := c.Get("userID")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录"})
		return
	}

	var items []cartItemResponse
	if err := model.DB.Table("cart_items c").
		Select("c.id, c.goods_id, c.spec, c.quantity, g.title, g.price, g.cover_url as image, g.stock, g.category").
		Joins("LEFT JOIN shop_goods g ON g.id = c.goods_id").
		Where("c.user_id = ?", userID).
		Order("c.id desc").
		Scan(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取购物车失败"})
		return
	}

	for i := range items {
		items[i].Price = formatGoodsPrice(int(items[i].Price))
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": items})
}

// AddToCart 添加商品到购物车
func AddToCart(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "数据库未连接"})
		return
	}

	userID, ok := c.Get("userID")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录"})
		return
	}

	var req struct {
		GoodsID  uint64 `json:"goods_id" binding:"required"`
		Spec     string `json:"spec"`
		Quantity int    `json:"quantity" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	var goods model.ShopGoods
	if err := model.DB.Where("id = ? AND status = ?", req.GoodsID, 1).First(&goods).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "商品不存在"})
		return
	}

	var item model.CartItem
	err := model.DB.Where("user_id = ? AND goods_id = ? AND spec = ?", userID, req.GoodsID, req.Spec).First(&item).Error
	if err == nil {
		item.Quantity += req.Quantity
		if item.Quantity > 99 {
			item.Quantity = 99
		}
		if err := model.DB.Save(&item).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "加入购物车失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": item})
		return
	}

	item = model.CartItem{
		UserID:   userID.(uint64),
		GoodsID:  req.GoodsID,
		Spec:     req.Spec,
		Quantity: req.Quantity,
	}

	if err := model.DB.Create(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "加入购物车失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": item})
}

// UpdateCartItem 更新购物车数量
func UpdateCartItem(c *gin.Context) {
	userID, ok := c.Get("userID")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "购物车ID无效"})
		return
	}

	var req struct {
		Quantity int `json:"quantity" binding:"required,min=1,max=99"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	result := model.DB.Model(&model.CartItem{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("quantity", req.Quantity)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新购物车失败"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "购物车商品不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success"})
}

// DeleteCartItem 删除购物车商品
func DeleteCartItem(c *gin.Context) {
	userID, ok := c.Get("userID")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "购物车ID无效"})
		return
	}

	result := model.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&model.CartItem{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "删除购物车失败"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "购物车商品不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success"})
}
