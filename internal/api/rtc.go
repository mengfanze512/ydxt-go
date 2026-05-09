package api

import (
	"context"
	"fmt"
	"net/http"
	"time"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

type ApplyMicRequest struct {
	LessonID uint64 `json:"lesson_id" binding:"required"`
}

// 连麦举手申请（学员端调用）
func ApplyMic(c *gin.Context) {
	var req ApplyMicRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录"})
		return
	}
	studentID := userIDVal.(uint64)

	// Redis 中使用 ZSet 存储举手队列，按时间戳排序
	queueKey := fmt.Sprintf("live:%d:mic_queue", req.LessonID)
	
	err := model.RDB.ZAdd(context.Background(), queueKey, &redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: studentID,
	}).Err()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "举手失败，请重试"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "举手申请已发送，请等待老师同意",
	})
}

// 获取举手列表（教师端调用）
func GetMicQueue(c *gin.Context) {
	lessonID := c.Query("lesson_id")
	if lessonID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "缺少 lesson_id 参数"})
		return
	}

	queueKey := fmt.Sprintf("live:%s:mic_queue", lessonID)
	
	// 获取所有举手的学生 ID
	members, err := model.RDB.ZRange(context.Background(), queueKey, 0, -1).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取举手列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"queue": members,
		},
	})
}

type HandleMicRequest struct {
	LessonID  uint64 `json:"lesson_id" binding:"required"`
	StudentID uint64 `json:"student_id" binding:"required"`
	Action    int    `json:"action"` // 1: 同意, 2: 拒绝
}

// 处理举手申请（教师端调用）
func HandleMic(c *gin.Context) {
	var req HandleMicRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	queueKey := fmt.Sprintf("live:%d:mic_queue", req.LessonID)

	// 无论同意还是拒绝，先将学生从等待队列中移除
	err := model.RDB.ZRem(context.Background(), queueKey, req.StudentID).Err()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "处理失败"})
		return
	}

	if req.Action == 1 {
		// 同意上麦，将学生加入已上麦集合
		activeKey := fmt.Sprintf("live:%d:active_mics", req.LessonID)
		model.RDB.SAdd(context.Background(), activeKey, req.StudentID)
		
		// 此处可以配合信令系统（如声网 RTM 或 WebSocket）通知学生端开始推流
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已同意连麦"})
	} else {
		// 拒绝
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已拒绝连麦"})
	}
}

// 结束连麦/下麦（学生主动下麦或老师强制下麦）
func EndMic(c *gin.Context) {
	var req HandleMicRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	activeKey := fmt.Sprintf("live:%d:active_mics", req.LessonID)
	model.RDB.SRem(context.Background(), activeKey, req.StudentID)

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "连麦已结束",
	})
}
