package api

import (
	"net/http"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
)

// GetSheetMusics 获取曲谱列表
func GetSheetMusics(c *gin.Context) {
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
