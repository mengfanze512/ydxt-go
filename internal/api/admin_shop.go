package api

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
)

type adminShopGoodsRequest struct {
	Title         string  `json:"title" binding:"required"`
	Description   string  `json:"description"`
	Price         float64 `json:"price"`
	OriginalPrice float64 `json:"original_price"`
	Stock         int     `json:"stock"`
	Sales         int     `json:"sales"`
	CoverURL      string  `json:"cover_url"`
	Category      string  `json:"category"`
	Status        int8    `json:"status"`
}

func sanitizeGoodsStatus(status int8) int8 {
	if status == 0 {
		return 0
	}
	return 1
}

func formatAdminShopGoodsList(c *gin.Context, goods []model.ShopGoods) {
	result := make([]shopGoodsResponse, 0, len(goods))
	for _, item := range goods {
		result = append(result, toShopGoodsResponse(item))
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": result})
}

// AdminGetShopGoods 获取后台商品列表
func AdminGetShopGoods(c *gin.Context) {
	if !requireAdminAccount(c) {
		return
	}
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": []shopGoodsResponse{}})
		return
	}

	var goods []model.ShopGoods
	query := model.DB.Order("created_at desc")

	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("title LIKE ? OR description LIKE ?", like, like)
	}
	if category := strings.TrimSpace(c.Query("category")); category != "" && category != "全部" {
		query = query.Where("category = ?", category)
	}
	if rawStatus := strings.TrimSpace(c.Query("status")); rawStatus != "" {
		if status, err := strconv.Atoi(rawStatus); err == nil && (status == 0 || status == 1) {
			query = query.Where("status = ?", status)
		}
	}

	if err := query.Find(&goods).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取商品列表失败"})
		return
	}

	formatAdminShopGoodsList(c, goods)
}

// AdminCreateShopGoods 新增商品
func AdminCreateShopGoods(c *gin.Context) {
	if !requireAdminAccount(c) {
		return
	}

	var req adminShopGoodsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	goods := model.ShopGoods{
		Title:         strings.TrimSpace(req.Title),
		Description:   strings.TrimSpace(req.Description),
		Price:         int(math.Round(req.Price * 100)),
		OriginalPrice: int(math.Round(req.OriginalPrice * 100)),
		Stock:         req.Stock,
		Sales:         req.Sales,
		CoverURL:      strings.TrimSpace(req.CoverURL),
		Category:      strings.TrimSpace(req.Category),
		Status:        sanitizeGoodsStatus(req.Status),
	}
	if goods.Stock < 0 {
		goods.Stock = 0
	}
	if goods.Sales < 0 {
		goods.Sales = 0
	}
	if goods.OriginalPrice < goods.Price {
		goods.OriginalPrice = goods.Price
	}

	if err := model.DB.Create(&goods).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "新增商品失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": toShopGoodsResponse(goods)})
}

// AdminUpdateShopGoods 编辑商品
func AdminUpdateShopGoods(c *gin.Context) {
	if !requireAdminAccount(c) {
		return
	}

	id, ok := parseUint(c, "id")
	if !ok {
		return
	}

	var req adminShopGoodsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	updates := map[string]interface{}{
		"title":          strings.TrimSpace(req.Title),
		"description":    strings.TrimSpace(req.Description),
		"price":          int(math.Round(req.Price * 100)),
		"original_price": int(math.Round(req.OriginalPrice * 100)),
		"stock":          max(req.Stock, 0),
		"sales":          max(req.Sales, 0),
		"cover_url":      strings.TrimSpace(req.CoverURL),
		"category":       strings.TrimSpace(req.Category),
		"status":         sanitizeGoodsStatus(req.Status),
	}
	if updates["original_price"].(int) < updates["price"].(int) {
		updates["original_price"] = updates["price"]
	}

	result := model.DB.Model(&model.ShopGoods{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新商品失败"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "商品不存在"})
		return
	}

	var goods model.ShopGoods
	_ = model.DB.Where("id = ?", id).First(&goods).Error
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": toShopGoodsResponse(goods)})
}

// AdminUpdateShopGoodsStatus 更新商品状态
func AdminUpdateShopGoodsStatus(c *gin.Context) {
	if !requireAdminAccount(c) {
		return
	}

	id, ok := parseUint(c, "id")
	if !ok {
		return
	}

	var req struct {
		Status int8 `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	result := model.DB.Model(&model.ShopGoods{}).Where("id = ?", id).Update("status", sanitizeGoodsStatus(req.Status))
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新商品状态失败"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "商品不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success"})
}

// AdminDeleteShopGoods 删除商品
func AdminDeleteShopGoods(c *gin.Context) {
	if !requireAdminAccount(c) {
		return
	}

	id, ok := parseUint(c, "id")
	if !ok {
		return
	}

	result := model.DB.Where("id = ?", id).Delete(&model.ShopGoods{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "删除商品失败"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "商品不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success"})
}
