package api

import (
	"net/http"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
)

// GetShopGoods 获取商城商品列表
func GetShopGoods(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": []model.ShopGoods{}})
		return
	}
	var goods []model.ShopGoods
	// 简单查询所有上架商品
	if err := model.DB.Where("status = ?", 1).Find(&goods).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取商品失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "success",
		"data": goods,
	})
}

// GetCartItems 获取用户的购物车列表
func GetCartItems(c *gin.Context) {
	userID, _ := c.Get("userID")
	var items []model.CartItem
	if err := model.DB.Where("user_id = ?", userID).Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取购物车失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "success",
		"data": items,
	})
}

// AddToCart 添加商品到购物车
func AddToCart(c *gin.Context) {
	userID, _ := c.Get("userID")
	var req struct {
		GoodsID  uint64 `json:"goods_id" binding:"required"`
		Spec     string `json:"spec"`
		Quantity int    `json:"quantity" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	item := model.CartItem{
		UserID:   userID.(uint64),
		GoodsID:  req.GoodsID,
		Spec:     req.Spec,
		Quantity: req.Quantity,
	}

	if err := model.DB.Create(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "加入购物车失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "success",
		"data": item,
	})
}
