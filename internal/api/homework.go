package api

import (
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
)

var (
	allowedHomeworkVideoExts = map[string]bool{
		".mp4": true, ".mov": true, ".m4v": true, ".avi": true,
		".mkv": true, ".webm": true, ".flv": true, ".wmv": true,
	}
	allowedHomeworkAudioExts = map[string]bool{
		".mp3": true, ".wav": true, ".aac": true, ".m4a": true,
		".ogg": true, ".flac": true, ".wma": true, ".amr": true,
	}
)

const maxHomeworkMediaSize = 500 << 20 // 500MB（前端直传，后端仅校验扩展名）

// GetHomeworkUploadCredential 学员端获取作业媒体上传凭证（前端直传对象存储，绕过 HTTP 网关大小限制）
func GetHomeworkUploadCredential(c *gin.Context) {
	ext := strings.ToLower(strings.TrimSpace(c.Query("ext")))
	if ext == "" {
		ext = ".mp4"
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	mediaType := ""
	if allowedHomeworkVideoExts[ext] {
		mediaType = "video"
	} else if allowedHomeworkAudioExts[ext] {
		mediaType = "audio"
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "仅支持视频(mp4/mov/m4v/avi/mkv/webm/flv/wmv)或音频(mp3/wav/aac/m4a/ogg/flac/wma/amr)格式",
		})
		return
	}

	cfg, err := loadStorageConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": fmt.Sprintf("上传配置缺失: %v", err)})
		return
	}

	accessToken, err := getWechatAccessToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": fmt.Sprintf("获取 access_token 失败: %v", err)})
		return
	}

	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	cloudPath := fmt.Sprintf("homework/%s/%d_%06d%s",
		time.Now().Format("20060102"),
		time.Now().UnixNano(),
		rnd.Intn(1000000),
		ext,
	)

	credential, err := getWechatUploadCredential(accessToken, cfg.EnvID, cloudPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": fmt.Sprintf("获取上传凭证失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"url":           credential.URL,
			"authorization": credential.Authorization,
			"token":         credential.Token,
			"cos_file_id":   credential.COSFileID,
			"file_id":       credential.FileID,
			"cloud_path":    cloudPath,
			"env_id":        cfg.EnvID,
			"media_type":    mediaType,
		},
	})
}

type submitHomeworkRequest struct {
	TaskID      uint64 `json:"task_id" binding:"required"`
	MediaFileID string `json:"media_file_id" binding:"required"`
	MediaType   string `json:"media_type" binding:"required,oneof=video audio"`
	TextContent string `json:"text_content"`
}

// SubmitHomework 学员提交作业（上传媒体 fileID + 文字说明）
func SubmitHomework(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "数据库未连接"})
		return
	}

	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}

	var req submitHomeworkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	// 校验任务是否存在且为作业类型
	var task model.LearningTask
	if err := model.DB.Where("id = ?", req.TaskID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "任务不存在"})
		return
	}

	now := time.Now()
	submission := model.HomeworkSubmission{
		TaskID:      req.TaskID,
		UserID:      userID,
		MediaFileID: strings.TrimSpace(req.MediaFileID),
		MediaType:   req.MediaType,
		TextContent: req.TextContent,
		Status:      "submitted",
		SubmittedAt: &now,
	}

	if err := model.DB.Create(&submission).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "提交作业失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": submission,
	})
}

// GetMyHomeworkSubmissions 学员查看自己的作业提交记录
func GetMyHomeworkSubmissions(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": []model.HomeworkSubmission{}})
		return
	}

	userID, ok := getUserIDFromContext(c)
	if !ok {
		return
	}

	var submissions []model.HomeworkSubmission
	query := model.DB.Where("user_id = ?", userID)
	if taskID := c.Query("task_id"); taskID != "" {
		query = query.Where("task_id = ?", taskID)
	}
	if err := query.Order("id desc").Find(&submissions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取作业提交记录失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": submissions})
}

// GetHomeworkSubmissions 管理端查看作业提交记录
func GetHomeworkSubmissions(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": []model.HomeworkSubmission{}})
		return
	}

	query := model.DB.Model(&model.HomeworkSubmission{})
	if scoped, teacherID := currentTeacherScope(c); scoped {
		query = query.Where("task_id IN (?)", model.DB.Table("learning_tasks t").Select("t.id").Joins("JOIN courses c ON c.id = t.course_id").Where("c.teacher_id = ?", teacherID))
	}
	if taskID := c.Query("task_id"); taskID != "" {
		query = query.Where("task_id = ?", taskID)
	}
	if userID := c.Query("user_id"); userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	var submissions []model.HomeworkSubmission
	if err := query.Order("id desc").Find(&submissions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取作业提交记录失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": submissions})
}

// GradeHomework shares the same ownership and score validation as teacher grading.
func GradeHomework(c *gin.Context) {
	GradeTeacherHomework(c)
}

// GetHomeworkMediaPlayURL 获取作业媒体的临时播放/下载链接
func GetHomeworkMediaPlayURL(c *gin.Context) {
	fileID := strings.TrimSpace(c.Query("file_id"))
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "缺少 file_id 参数"})
		return
	}
	var count int64
	query := model.DB.Model(&model.HomeworkSubmission{}).Where("media_file_id = ?", fileID)
	if scoped, teacherID := currentTeacherScope(c); scoped {
		query = query.Where("task_id IN (?)", model.DB.Table("learning_tasks t").Select("t.id").Joins("JOIN courses c ON c.id = t.course_id").Where("c.teacher_id = ?", teacherID))
	} else if account, _ := c.Get("accountType"); account != accountTypeAdmin {
		userID, _ := c.Get("userID")
		query = query.Where("user_id = ?", userID)
	}
	if err := query.Count(&count).Error; err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "校验附件权限失败: " + err.Error()})
		return
	}
	if count == 0 {
		c.JSON(403, gin.H{"code": 403, "msg": "附件不存在或不属于可访问的作业"})
		return
	}

	cfg, err := loadStorageConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": fmt.Sprintf("存储配置缺失: %v", err)})
		return
	}

	accessToken, err := getWechatAccessToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": fmt.Sprintf("获取 access_token 失败: %v", err)})
		return
	}

	downloadURL, err := getWechatDownloadURL(accessToken, cfg.EnvID, fileID, 7200)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": fmt.Sprintf("获取媒体链接失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"play_url": downloadURL,
			"file_id":  fileID,
		},
	})
}
