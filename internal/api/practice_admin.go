package api

import (
	"net/http"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
)

// GetPracticeRecords 获取学员AI练习记录
func GetPracticeRecords(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": []model.PracticeRecord{}})
		return
	}
	var records []model.PracticeRecord
	// 后台管理端可以查所有
	if err := model.DB.Order("created_at desc").Find(&records).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取练习记录失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "success",
		"data": records,
	})
}
