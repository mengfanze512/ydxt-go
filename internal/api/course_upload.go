package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tencentyun/cos-go-sdk-v5"
)

const maxCourseImageSize = 10 << 20 // 10MB

type storageConfig struct {
	SecretID     string
	SecretKey    string
	SessionToken string
	Bucket       string
	Region       string
	AppID        string
	Source       string
}

func loadStorageConfig() (storageConfig, error) {
	var cfg storageConfig

	// 方案A：用户显式配置了长期密钥（不带Token）
	if os.Getenv("COS_SECRET_ID") != "" && os.Getenv("COS_SECRET_KEY") != "" {
		cfg.SecretID = os.Getenv("COS_SECRET_ID")
		cfg.SecretKey = os.Getenv("COS_SECRET_KEY")
		cfg.SessionToken = os.Getenv("COS_SESSION_TOKEN") // 长期密钥通常为空
		cfg.Bucket = os.Getenv("COS_BUCKET")
		cfg.Region = os.Getenv("COS_REGION")
		cfg.AppID = os.Getenv("COS_APP_ID")
		cfg.Source = "explicit-cos-env"
	} else {
		// 方案B：微信云托管上下文自动注入的临时密钥
		cfg.SecretID = firstNonEmpty("TENCENTCLOUD_SECRETID", "SECRETID")
		cfg.SecretKey = firstNonEmpty("TENCENTCLOUD_SECRETKEY", "SECRETKEY")
		cfg.SessionToken = firstNonEmpty("TENCENTCLOUD_SESSIONTOKEN", "SESSIONTOKEN", "TOKEN")
		cfg.Bucket = firstNonEmpty("TCB_STORAGE_BUCKET")
		cfg.Region = firstNonEmpty("TCB_REGION", "REGION")
		cfg.AppID = firstNonEmpty("TCB_APPID", "APPID")
		cfg.Source = "wxcloud-context-env"
	}

	// 再从微信云托管上下文补齐（TCB_CONTEXT_CNFG）
	ctxRaw := os.Getenv("TCB_CONTEXT_CNFG")
	if ctxRaw != "" {
		var ctx map[string]interface{}
		if err := json.Unmarshal([]byte(ctxRaw), &ctx); err == nil {
			if cfg.Bucket == "" {
				cfg.Bucket = findValueByKeys(ctx, []string{"cosbucket", "bucket", "storagename", "storagebucket", "defaultbucket"})
			}
			if cfg.Region == "" {
				cfg.Region = findValueByKeys(ctx, []string{"region", "cosregion"})
			}
			if cfg.AppID == "" {
				cfg.AppID = findValueByKeys(ctx, []string{"appid", "uin"})
			}
		}
	}

	// 若 Bucket 未带 appid，则自动拼接（云托管用户常只拿到短桶名）
	if cfg.Bucket != "" && !strings.Contains(cfg.Bucket, "-") && cfg.AppID != "" {
		cfg.Bucket = fmt.Sprintf("%s-%s", cfg.Bucket, cfg.AppID)
	}

	if cfg.SecretID == "" || cfg.SecretKey == "" || cfg.Bucket == "" || cfg.Region == "" {
		return storageConfig{}, errors.New("缺失存储配置。请检查环境变量 COS_SECRET_ID/KEY/BUCKET/REGION")
	}

	return cfg, nil
}

func firstNonEmpty(keys ...string) string {
	for _, key := range keys {
		if val := strings.TrimSpace(os.Getenv(key)); val != "" {
			return val
		}
	}
	return ""
}

func findValueByKeys(data interface{}, candidates []string) string {
	normalizedCandidates := map[string]struct{}{}
	for _, key := range candidates {
		normalizedCandidates[strings.ToLower(strings.TrimSpace(key))] = struct{}{}
	}

	var walk func(v interface{}) string
	walk = func(v interface{}) string {
		switch node := v.(type) {
		case map[string]interface{}:
			for key, val := range node {
				k := strings.ToLower(strings.TrimSpace(key))
				if _, ok := normalizedCandidates[k]; ok {
					if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
						return strings.TrimSpace(s)
					}
				}
				if nested := walk(val); nested != "" {
					return nested
				}
			}
		case []interface{}:
			for _, item := range node {
				if nested := walk(item); nested != "" {
					return nested
				}
			}
		}
		return ""
	}
	return walk(data)
}

// AdminUploadCourseImage 管理端上传课程图片到对象存储 courses_images 目录
func AdminUploadCourseImage(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请选择要上传的图片"})
		return
	}
	if fileHeader.Size <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "图片不能为空"})
		return
	}
	if fileHeader.Size > maxCourseImageSize {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "图片大小不能超过10MB"})
		return
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if ext == "" {
		ext = ".jpg"
	}
	allowed := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".webp": true,
		".gif":  true,
	}
	if !allowed[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "仅支持 jpg/jpeg/png/webp/gif 格式"})
		return
	}

	cfg, err := loadStorageConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  fmt.Sprintf("对象存储配置缺失: %v", err),
		})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "读取上传文件失败"})
		return
	}
	defer file.Close()

	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	objectKey := fmt.Sprintf("courses_images/%s/%d_%06d%s",
		time.Now().Format("20060102"),
		time.Now().UnixNano(),
		rnd.Intn(1000000),
		ext,
	)

	baseURL := fmt.Sprintf("https://%s.cos.%s.myqcloud.com", cfg.Bucket, cfg.Region)
	u, err := url.Parse(baseURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "对象存储地址配置错误"})
		return
	}

	cosClient := cos.NewClient(
		&cos.BaseURL{BucketURL: u},
		&http.Client{
			Transport: &cos.AuthorizationTransport{
				SecretID:     cfg.SecretID,
				SecretKey:    cfg.SecretKey,
				SessionToken: cfg.SessionToken,
			},
		},
	)

	if _, err = cosClient.Object.Put(context.Background(), objectKey, file, nil); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  fmt.Sprintf("上传失败[%s]: %v", cfg.Source, err),
		})
		return
	}

	imageURL := strings.TrimRight(baseURL, "/") + "/" + objectKey
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"url":    imageURL,
			"key":    objectKey,
			"source": cfg.Source,
		},
	})
}
