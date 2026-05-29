package api

import (
	"net/http"
	"strconv"
	"strings"
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
	ID       uint64  `json:"id"`
	ItemType string  `json:"item_type"`
	GoodsID  uint64  `json:"goods_id"`
	Title    string  `json:"title"`
	Spec     string  `json:"spec"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
	Image    string  `json:"image"`
	Stock    int     `json:"stock"`
	Category string  `json:"category"`
}

const (
	cartItemTypeShop   = "shop"
	cartItemTypeCourse = "course"
	cartItemTypeSheet  = "sheet"
)

func normalizeCartItemType(raw string) string {
	switch raw {
	case cartItemTypeCourse:
		return cartItemTypeCourse
	case cartItemTypeSheet:
		return cartItemTypeSheet
	default:
		return cartItemTypeShop
	}
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

	var cartItems []model.CartItem
	if err := model.DB.Where("user_id = ?", userID).Order("id desc").Find(&cartItems).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取购物车失败"})
		return
	}

	items := make([]cartItemResponse, 0, len(cartItems))
	for _, cartItem := range cartItems {
		itemType := normalizeCartItemType(cartItem.ItemType)
		item := cartItemResponse{
			ID:       cartItem.ID,
			ItemType: itemType,
			GoodsID:  cartItem.GoodsID,
			Spec:     cartItem.Spec,
			Quantity: cartItem.Quantity,
		}

		switch itemType {
		case cartItemTypeShop:
			var goods model.ShopGoods
			if err := model.DB.Where("id = ? AND status = ?", cartItem.GoodsID, 1).First(&goods).Error; err != nil {
				continue
			}
			item.Title = goods.Title
			item.Price = formatGoodsPrice(goods.Price)
			item.Image = strings.TrimSpace(goods.CoverURL)
			item.Stock = goods.Stock
			item.Category = goods.Category
		case cartItemTypeCourse:
			var course model.Course
			if err := model.DB.Where("id = ? AND status = ?", cartItem.GoodsID, 1).First(&course).Error; err != nil {
				continue
			}
			item.Title = course.Title
			item.Price = course.Price
			item.Image = strings.TrimSpace(course.Cover)
			item.Stock = 1
			item.Category = "课程"
			item.Quantity = 1
			item.Spec = ""
		case cartItemTypeSheet:
			if isLegacySheetMusicSchema() {
				var sheet legacySheetMusicRecord
				if err := model.DB.Table("sheet_musics").
					Select(legacySheetMusicSelectFields()).
					Where("id = ? AND is_deleted = ? AND status = ?", cartItem.GoodsID, 0, 1).
					First(&sheet).Error; err != nil {
					continue
				}
				sheetData := toLegacySheetMusicResponse(sheet)
				item.Title = sheetData.Title
				item.Price = sheetData.Price
				item.Image = strings.TrimSpace(sheetData.CoverURL)
			} else {
				var sheet model.SheetMusic
				if err := model.DB.Where("id = ?", cartItem.GoodsID).First(&sheet).Error; err != nil {
					continue
				}
				sheetData := toSheetMusicResponse(sheet)
				item.Title = sheetData.Title
				item.Price = sheetData.Price
				item.Image = strings.TrimSpace(sheetData.CoverURL)
			}
			item.Stock = 1
			item.Category = "曲谱"
			item.Quantity = 1
			item.Spec = ""
		default:
			continue
		}

		items = append(items, item)
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
		ItemType string `json:"item_type"`
		GoodsID  uint64 `json:"goods_id" binding:"required"`
		Spec     string `json:"spec"`
		Quantity int    `json:"quantity" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	itemType := normalizeCartItemType(req.ItemType)
	quantity := req.Quantity
	spec := req.Spec
	if itemType != cartItemTypeShop {
		quantity = 1
		spec = ""
	}

	switch itemType {
	case cartItemTypeShop:
		var goods model.ShopGoods
		if err := model.DB.Where("id = ? AND status = ?", req.GoodsID, 1).First(&goods).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "商品不存在"})
			return
		}
	case cartItemTypeCourse:
		var course model.Course
		if err := model.DB.Where("id = ? AND status = ?", req.GoodsID, 1).First(&course).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "课程不存在"})
			return
		}
	case cartItemTypeSheet:
		if isLegacySheetMusicSchema() {
			var sheet legacySheetMusicRecord
			if err := model.DB.Table("sheet_musics").
				Select(legacySheetMusicSelectFields()).
				Where("id = ? AND is_deleted = ? AND status = ?", req.GoodsID, 0, 1).
				First(&sheet).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "曲谱不存在"})
				return
			}
			if hasSheetMusicColumn("price") && sheet.Price < 0 {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "曲谱价格异常"})
				return
			}
			break
		}
		var sheet model.SheetMusic
		if err := model.DB.Where("id = ?", req.GoodsID).First(&sheet).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "曲谱不存在"})
			return
		}
		if normalizeSheetFree(sheet.IsFree) == 0 && sheet.Price <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "该曲谱价格未配置，暂不可加入购物车"})
			return
		}
	}

	var item model.CartItem
	err := model.DB.Where("user_id = ? AND item_type = ? AND goods_id = ? AND spec = ?", userID, itemType, req.GoodsID, spec).First(&item).Error
	if err == nil {
		if itemType == cartItemTypeShop {
			item.Quantity += quantity
			if item.Quantity > 99 {
				item.Quantity = 99
			}
		} else {
			item.Quantity = 1
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
		ItemType: itemType,
		GoodsID:  req.GoodsID,
		Spec:     spec,
		Quantity: quantity,
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

	var item model.CartItem
	if err := model.DB.Where("id = ? AND user_id = ?", id, userID).First(&item).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "购物车商品不存在"})
		return
	}

	quantity := req.Quantity
	if normalizeCartItemType(item.ItemType) != cartItemTypeShop {
		quantity = 1
	}

	result := model.DB.Model(&model.CartItem{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("quantity", quantity)
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
