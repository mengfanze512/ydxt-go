package api

import (
	"net/http"
	"time"
	"yuedi_edu/internal/model"

	"github.com/gin-gonic/gin"
)

type StartLiveRequest struct {
	LessonID uint64 `json:"lesson_id" binding:"required"`
}

// StartLive 讲师开播接口
func StartLive(c *gin.Context) {
	var req StartLiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录"})
		return
	}
	teacherID := userIDVal.(uint64)

	// 1. 查找是否已经有该 lesson 的直播间
	var liveRoom model.LiveRoom
	err := model.DB.Where("lesson_id = ? AND teacher_id = ?", req.LessonID, teacherID).First(&liveRoom).Error
	now := time.Now()

	if err != nil {
		// 不存在则创建新的直播间记录
		liveRoom = model.LiveRoom{
			LessonID:        req.LessonID,
			TeacherID:       teacherID,
			Status:          1, // 直播中
			ActualStartTime: &now,
			OnlineCount:     0,
		}
		if err := model.DB.Create(&liveRoom).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "创建直播间失败"})
			return
		}
	} else {
		// 存在则更新状态为直播中
		if liveRoom.Status == 2 {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "该课程直播已结束"})
			return
		}
		liveRoom.Status = 1
		liveRoom.ActualStartTime = &now
		if err := model.DB.Save(&liveRoom).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新直播间状态失败"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "开播成功",
		"data": gin.H{
			"live_room_id": liveRoom.ID,
		},
	})
}

// EndLive 讲师下播接口
func EndLive(c *gin.Context) {
	roomID := c.Param("id")

	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录"})
		return
	}
	teacherID := userIDVal.(uint64)

	var liveRoom model.LiveRoom
	if err := model.DB.Where("id = ? AND teacher_id = ?", roomID, teacherID).First(&liveRoom).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "未找到相关直播间或无权限"})
		return
	}

	if liveRoom.Status != 1 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "该直播间不在直播中"})
		return
	}

	now := time.Now()
	liveRoom.Status = 2 // 已结束
	liveRoom.ActualEndTime = &now

	if err := model.DB.Save(&liveRoom).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "下播失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "下播成功",
	})
}

// GetLiveRoomStatus 获取直播间状态 (供学生端轮询或初始化获取)
func GetLiveRoomStatus(c *gin.Context) {
	lessonID := c.Query("lesson_id")
	if lessonID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "缺少 lesson_id 参数"})
		return
	}

	var liveRoom model.LiveRoom
	if err := model.DB.Where("lesson_id = ?", lessonID).First(&liveRoom).Error; err != nil {
		// 未找到记录，说明尚未开播
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"msg":  "success",
			"data": gin.H{
				"status": 0,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"room_id":          liveRoom.ID,
			"status":           liveRoom.Status,
			"online_count":     liveRoom.OnlineCount,
			"current_sheet_id": liveRoom.CurrentSheetID,
		},
	})
}
