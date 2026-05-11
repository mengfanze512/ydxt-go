package api

import (
	"context"
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

	secretID := os.Getenv("COS_SECRET_ID")
	secretKey := os.Getenv("COS_SECRET_KEY")
	bucket := os.Getenv("COS_BUCKET")
	region := os.Getenv("COS_REGION")
	if secretID == "" || secretKey == "" || bucket == "" || region == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "对象存储配置缺失，请设置 COS_SECRET_ID/COS_SECRET_KEY/COS_BUCKET/COS_REGION"})
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

	baseURL := fmt.Sprintf("https://%s.cos.%s.myqcloud.com", bucket, region)
	u, err := url.Parse(baseURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "对象存储地址配置错误"})
		return
	}

	cosClient := cos.NewClient(
		&cos.BaseURL{BucketURL: u},
		&http.Client{
			Transport: &cos.AuthorizationTransport{
				SecretID:  secretID,
				SecretKey: secretKey,
			},
		},
	)

	if _, err = cosClient.Object.Put(context.Background(), objectKey, file, nil); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "上传对象存储失败"})
		return
	}

	imageURL := strings.TrimRight(baseURL, "/") + "/" + objectKey
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"url": imageURL,
			"key": objectKey,
		},
	})
}
