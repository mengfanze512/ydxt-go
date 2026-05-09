package api

import (
	"net/http"
	"time"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
)

// DoCheckin 执行打卡
func DoCheckin(c *gin.Context) {
	userID, _ := c.Get("userID")
	today := time.Now().Format("2006-01-02")

	// 检查今日是否已打卡
	var existing model.UserCheckin
	if err := model.DB.Where("user_id = ? AND checkin_date = ?", userID, today).First(&existing).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "今日已打卡"})
		return
	}

	checkin := model.UserCheckin{
		UserID:         userID.(uint64),
		CheckinDate:    today,
		ContinuousDays: 1, // 实际逻辑需要查询昨天的打卡记录来累加
	}

	if err := model.DB.Create(&checkin).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "打卡失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "打卡成功",
		"data": checkin,
	})
}
