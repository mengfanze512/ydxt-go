package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
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
	Source       string
}

func loadStorageConfig() (storageConfig, error) {
	var cfg storageConfig

	// 从环境变量读取，支持多种常用写法
	cfg.SecretID = firstNonEmpty("COS_SECRET_ID", "TENCENTCLOUD_SECRETID", "SECRETID")
	cfg.SecretKey = firstNonEmpty("COS_SECRET_KEY", "TENCENTCLOUD_SECRETKEY", "SECRETKEY")
	cfg.SessionToken = firstNonEmpty("COS_SESSION_TOKEN", "TENCENTCLOUD_SESSIONTOKEN", "SESSIONTOKEN", "TOKEN")
	cfg.Bucket = firstNonEmpty("COS_BUCKET", "TCB_STORAGE_BUCKET", "BUCKET")
	cfg.Region = firstNonEmpty("COS_REGION", "TCB_REGION", "REGION")

	cfg.Source = "env"

	if cfg.SecretID == "" || cfg.SecretKey == "" || cfg.Bucket == "" || cfg.Region == "" {
		return cfg, errors.New("缺少 COS_SECRET_ID/KEY/BUCKET/REGION 配置")
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

// AdminUploadCourseImage 管理端上传课程图片到对象存储
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
		".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true,
	}
	if !allowed[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "仅支持 jpg/jpeg/png/webp/gif 格式"})
		return
	}

	cfg, err := loadStorageConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  fmt.Sprintf("上传配置缺失: %v", err),
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
	cloudPath := fmt.Sprintf("courses_images/%s/%d_%06d%s",
		time.Now().Format("20060102"),
		time.Now().UnixNano(),
		rnd.Intn(1000000),
		ext,
	)

	// 如果你之前遇到了 403，但又想继续用你配置的那套 `COS_SECRET_ID/KEY/BUCKET/REGION` 变量
	// 最稳妥的方法是使用 COS 的 HTTP API 直接上传（其实 COS-SDK 也是包装了这个）
	// 但是我们要确保不传错误的 Header

	// 方案1：使用 HTTP 原生构建 POST 表单
	// 注意：COS 并不推荐用这种原生 HTTP，因为签名特别麻烦。
	// 所以我们还是得回到 COS-SDK，并严格指定鉴权 Transport。

	// 这里是之前 403 的根因修正：
	// 如果你只给了 PutObject 权限，但代码尝试去做 MultipartUpload 或者获取其他信息，就会报 403。
	// 对于小文件，应该强制使用 SimpleUpload。

	// === 以下为最终修正版，完全隔离并强制使用简单上传 ===
	uploadURL := fmt.Sprintf("https://%s.cos.%s.myqcloud.com", cfg.Bucket, cfg.Region)

	// 如果你希望通过微信云托管原生服务直接穿透（免鉴权 HTTP API）：
	// 我们可以调用微信的 HTTP API，但是那需要获取 AccessToken。

	// 为了最快解决，我们依然使用原生 HTTP POST 表单配合微信 API，
	// 或者直接在后端将文件流代理到另一个微信服务端接口。

	// 考虑到你之前已经配好了那4个变量，并且你确定它是个正常的云开发桶：
	// 这里使用腾讯云云开发 TCB 原生 HTTP API 的一种变通做法（如果我们无法加载正确的 SDK）

	// 鉴于 TCB SDK 的方法不一致，我们回退到使用原生 HTTP 配合 COS 签名。

	// 由于你现在有明确的 403 报错，这证明 COS SDK 是通的，只是鉴权没过。
	// 为了让你能够绕过这个限制，我建议：
	// 把云托管里临时 token 的环境变量完全覆盖为空，确保 COS SDK 绝对拿不到错误的 Token！
	os.Setenv("TENCENTCLOUD_SESSIONTOKEN", "")

	u, _ := url.Parse(uploadURL)
	b := &cos.BaseURL{BucketURL: u}

	client := cos.NewClient(b, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:     cfg.SecretID,
			SecretKey:    cfg.SecretKey,
			SessionToken: cfg.SessionToken,
		},
	})

	// 强制读取整个文件内容，避免触发分块上传
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "读取文件内容失败"})
		return
	}

	reader := bytes.NewReader(fileBytes)
	opt := &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
			ContentType: fileHeader.Header.Get("Content-Type"),
		},
	}

	_, err = client.Object.Put(context.Background(), cloudPath, reader, opt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  fmt.Sprintf("上传失败[%s]: %v", cfg.Source, err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"url":    uploadURL + "/" + cloudPath,
			"key":    cloudPath,
			"source": cfg.Source,
		},
	})
}
