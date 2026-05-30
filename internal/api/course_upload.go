package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const maxCourseImageSize = 10 << 20 // 10MB

type storageConfig struct {
	SecretID     string
	SecretKey    string
	SessionToken string
	Bucket       string
	Region       string
	EnvID        string
	Source       string
}

// #region debug-point A:report-helper
func reportCourseUploadDebug(hypothesisID, traceID, msg string, data map[string]interface{}) {
	debugURL := "http://127.0.0.1:7777/event"
	sessionID := "course-image-upload-500"
	if content, err := os.ReadFile(".dbg/course-image-upload-500.env"); err == nil {
		for _, line := range strings.Split(string(content), "\n") {
			if strings.HasPrefix(line, "DEBUG_SERVER_URL=") {
				debugURL = strings.TrimSpace(strings.TrimPrefix(line, "DEBUG_SERVER_URL="))
			}
			if strings.HasPrefix(line, "DEBUG_SESSION_ID=") {
				sessionID = strings.TrimSpace(strings.TrimPrefix(line, "DEBUG_SESSION_ID="))
			}
		}
	}
	payload := map[string]interface{}{
		"sessionId":    sessionID,
		"runId":        "pre-fix",
		"hypothesisId": hypothesisID,
		"location":     "internal/api/course_upload.go",
		"msg":          "[DEBUG] " + msg,
		"data":         data,
		"traceId":      traceID,
		"ts":           time.Now().UnixMilli(),
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, debugURL, bytes.NewReader(body))
	if req != nil {
		req.Header.Set("Content-Type", "application/json")
		_, _ = http.DefaultClient.Do(req)
	}
}
// #endregion

func loadStorageConfig() (storageConfig, error) {
	var cfg storageConfig

	cfg.SecretID = firstNonEmpty("COS_SECRET_ID", "TENCENTCLOUD_SECRETID", "SECRETID")
	cfg.SecretKey = firstNonEmpty("COS_SECRET_KEY", "TENCENTCLOUD_SECRETKEY", "SECRETKEY")
	cfg.SessionToken = firstNonEmpty("COS_SESSION_TOKEN", "TENCENTCLOUD_SESSIONTOKEN", "SESSIONTOKEN", "TOKEN")
	cfg.Bucket = firstNonEmpty("COS_BUCKET", "TCB_STORAGE_BUCKET", "BUCKET")
	cfg.Region = firstNonEmpty("COS_REGION", "TCB_REGION", "REGION")
	cfg.EnvID = firstNonEmpty("CLOUD_ENV_ID", "TCB_ENV", "TCB_ENVID", "WX_CLOUD_ENV", "ENV_ID")

	cfg.Source = "env"

	if cfg.EnvID == "" && cfg.Bucket != "" {
		cfg.EnvID = inferEnvIDFromBucket(cfg.Bucket)
		if cfg.EnvID != "" {
			cfg.Source = "env+bucket"
		}
	}

	if cfg.SecretID == "" || cfg.SecretKey == "" || cfg.EnvID == "" {
		return cfg, errors.New("缺少 COS_SECRET_ID/KEY 或 CLOUD_ENV_ID/TCB_ENV 配置")
	}

	return cfg, nil
}

func inferEnvIDFromBucket(bucket string) string {
	matched := regexp.MustCompile(`^[^-]+-(.+)-\d+$`).FindStringSubmatch(strings.TrimSpace(bucket))
	if len(matched) == 2 {
		return strings.TrimSpace(matched[1])
	}
	return ""
}

type cloudBaseUploadResult struct {
	FileID      string `json:"fileID"`
	TempFileURL string `json:"tempFileURL"`
}

func uploadCourseImageByCloudBase(localFilePath, cloudPath string, cfg storageConfig) (cloudBaseUploadResult, error) {
	cmd := exec.Command("node", "/app/cloudbase-uploader/upload.mjs", localFilePath, cloudPath, cfg.EnvID)
	cmd.Env = append(os.Environ(),
		"COS_SECRET_ID="+cfg.SecretID,
		"COS_SECRET_KEY="+cfg.SecretKey,
		"COS_SESSION_TOKEN="+cfg.SessionToken,
		"TENCENTCLOUD_SECRETID="+cfg.SecretID,
		"TENCENTCLOUD_SECRETKEY="+cfg.SecretKey,
		"TENCENTCLOUD_SESSIONTOKEN="+cfg.SessionToken,
		"CLOUD_ENV_ID="+cfg.EnvID,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return cloudBaseUploadResult{}, fmt.Errorf("cloudbase uploader failed: %s", strings.TrimSpace(string(output)))
	}

	var result cloudBaseUploadResult
	if err := json.Unmarshal(output, &result); err != nil {
		return cloudBaseUploadResult{}, fmt.Errorf("parse cloudbase uploader result failed: %v, raw: %s", err, strings.TrimSpace(string(output)))
	}
	if strings.TrimSpace(result.FileID) == "" {
		return cloudBaseUploadResult{}, fmt.Errorf("cloudbase uploader returned empty fileID")
	}
	return result, nil
}

func firstNonEmpty(keys ...string) string {
	for _, key := range keys {
		if val := strings.TrimSpace(os.Getenv(key)); val != "" {
			return val
		}
	}
	return ""
}

// AdminUploadCourseImage 管理端上传课程图片到对象存储
func AdminUploadCourseImage(c *gin.Context) {
	traceID := fmt.Sprintf("course-upload-%d", time.Now().UnixNano())
	fileHeader, err := c.FormFile("file")
	if err != nil {
		// #region debug-point C:form-file-error
		reportCourseUploadDebug("C", traceID, "form file parse failed", map[string]interface{}{"error": err.Error()})
		// #endregion
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请选择要上传的图片"})
		return
	}
	if fileHeader.Size <= 0 {
		// #region debug-point E:empty-file
		reportCourseUploadDebug("E", traceID, "file size invalid", map[string]interface{}{"size": fileHeader.Size, "filename": fileHeader.Filename})
		// #endregion
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "图片不能为空"})
		return
	}
	if fileHeader.Size > maxCourseImageSize {
		// #region debug-point E:file-too-large
		reportCourseUploadDebug("E", traceID, "file too large", map[string]interface{}{"size": fileHeader.Size, "filename": fileHeader.Filename})
		// #endregion
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "图片大小不能超过10MB"})
		return
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if ext == "" {
		ext = ".jpg"
	}
	allowed := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true,
	}
	if !allowed[ext] {
		// #region debug-point E:file-type-invalid
		reportCourseUploadDebug("E", traceID, "file extension not allowed", map[string]interface{}{"ext": ext, "filename": fileHeader.Filename, "contentType": fileHeader.Header.Get("Content-Type")})
		// #endregion
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "仅支持 jpg/jpeg/png/webp/gif 格式"})
		return
	}

	cfg, err := loadStorageConfig()
	if err != nil {
		// #region debug-point A:storage-config-error
		reportCourseUploadDebug("A", traceID, "storage config missing", map[string]interface{}{"error": err.Error()})
		// #endregion
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  fmt.Sprintf("上传配置缺失: %v", err),
		})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		// #region debug-point C:file-open-error
		reportCourseUploadDebug("C", traceID, "open uploaded file failed", map[string]interface{}{"error": err.Error(), "filename": fileHeader.Filename})
		// #endregion
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "读取上传文件失败"})
		return
	}
	defer file.Close()

	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	cloudPath := fmt.Sprintf("courses_images/%s/%d_%06d%s",
		time.Now().Format("20060102"),
		time.Now().UnixNano(),
		rnd.Intn(1000000),
		ext,
	)

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		// #region debug-point C:file-read-error
		reportCourseUploadDebug("C", traceID, "read uploaded file failed", map[string]interface{}{"error": err.Error(), "filename": fileHeader.Filename})
		// #endregion
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "读取文件内容失败"})
		return
	}

	// #region debug-point A:upload-start
	reportCourseUploadDebug("A", traceID, "upload started", map[string]interface{}{
		"filename":      fileHeader.Filename,
		"size":          fileHeader.Size,
		"ext":           ext,
		"bucket":        cfg.Bucket,
		"region":        cfg.Region,
		"envId":         cfg.EnvID,
		"source":        cfg.Source,
		"hasSecretID":   cfg.SecretID != "",
		"hasSecretKey":  cfg.SecretKey != "",
		"hasSessionTok": strings.TrimSpace(cfg.SessionToken) != "",
		"cloudPath":     cloudPath,
	})
	// #endregion

	tempFile, err := os.CreateTemp("", "course-upload-*"+ext)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "创建临时文件失败"})
		return
	}
	tempFilePath := tempFile.Name()
	if _, err := tempFile.Write(fileBytes); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempFilePath)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "写入临时文件失败"})
		return
	}
	_ = tempFile.Close()
	defer os.Remove(tempFilePath)

	result, err := uploadCourseImageByCloudBase(tempFilePath, cloudPath, cfg)
	if err != nil {
		// #region debug-point B:put-object-error
		reportCourseUploadDebug("B", traceID, "put object failed", map[string]interface{}{
			"error":         err.Error(),
			"bucket":        cfg.Bucket,
			"region":        cfg.Region,
			"source":        cfg.Source,
			"hasSessionTok": strings.TrimSpace(cfg.SessionToken) != "",
			"cloudPath":     cloudPath,
		})
		// #endregion
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  fmt.Sprintf("上传失败[%s]: %v", cfg.Source, err),
		})
		return
	}

	// #region debug-point B:put-object-success
	reportCourseUploadDebug("B", traceID, "put object success", map[string]interface{}{"cloudPath": cloudPath, "url": result.TempFileURL, "fileID": result.FileID})
	// #endregion

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"url":    result.TempFileURL,
			"key":    cloudPath,
			"fileID": result.FileID,
			"source": cfg.Source,
		},
	})
}
