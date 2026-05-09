package api

import (
	"net/http"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
)

// GetSheetMusics 获取曲谱列表
func GetSheetMusics(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": []model.SheetMusic{}})
		return
	}
	var sheets []model.SheetMusic
	if err := model.DB.Find(&sheets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取曲谱失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "success",
		"data": sheets,
	})
}
