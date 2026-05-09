package api

import (
	"net/http"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
)

// GetSettlements 获取财务结算单列表 (Admin)
func GetSettlements(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": []model.FinanceSettlement{}})
		return
	}
	var settlements []model.FinanceSettlement
	if err := model.DB.Order("created_at desc").Find(&settlements).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取结算单失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "success",
		"data": settlements,
	})
}
