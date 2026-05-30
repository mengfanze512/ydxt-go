package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const maxCourseImageSize = 10 << 20 // 10MB
const defaultCloudEnvID = "prod-d8gxf4vm265aab5e2"

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

	if cfg.EnvID == "" && defaultCloudEnvID != "" {
		cfg.EnvID = defaultCloudEnvID
		cfg.Source = "project-default"
	}

	if cfg.EnvID == "" && cfg.Bucket != "" {
		cfg.EnvID = inferEnvIDFromBucket(cfg.Bucket)
		if cfg.EnvID != "" {
			cfg.Source = "env+bucket"
		}
	}

	if cfg.EnvID == "" {
		return cfg, errors.New("缺少 CLOUD_ENV_ID/TCB_ENV 配置")
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
	PreviewURL  string `json:"previewURL"`
}

type wechatUploadCredentialResp struct {
	ErrCode       int    `json:"errcode"`
	ErrMsg        string `json:"errmsg"`
	URL           string `json:"url"`
	Token         string `json:"token"`
	Authorization string `json:"authorization"`
	FileID        string `json:"file_id"`
	COSFileID     string `json:"cos_file_id"`
}

type wechatDownloadFileItem struct {
	FileID      string `json:"fileid"`
	DownloadURL string `json:"download_url"`
	Status      int    `json:"status"`
	ErrMsg      string `json:"errmsg"`
}

type wechatDownloadFileResp struct {
	ErrCode  int                    `json:"errcode"`
	ErrMsg   string                 `json:"errmsg"`
	FileList []wechatDownloadFileItem `json:"file_list"`
}

func postWechatCloudAPI(accessToken, apiPath string, payload interface{}, out interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化微信云接口请求失败: %w", err)
	}

	apiURL := "https://api.weixin.qq.com" + apiPath + "?access_token=" + url.QueryEscape(strings.TrimSpace(accessToken))
	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建微信云接口请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("调用微信云接口失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("调用微信云接口失败: HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("解析微信云接口响应失败: %v, raw: %s", err, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func getWechatUploadCredential(accessToken, envID, cloudPath string) (wechatUploadCredentialResp, error) {
	var resp wechatUploadCredentialResp
	err := postWechatCloudAPI(accessToken, "/tcb/uploadfile", map[string]string{
		"env":  strings.TrimSpace(envID),
		"path": strings.TrimSpace(cloudPath),
	}, &resp)
	if err != nil {
		return resp, err
	}
	if resp.ErrCode != 0 {
		return resp, fmt.Errorf("微信云获取上传凭证失败: errcode=%d errmsg=%s", resp.ErrCode, resp.ErrMsg)
	}
	if strings.TrimSpace(resp.URL) == "" || strings.TrimSpace(resp.FileID) == "" || strings.TrimSpace(resp.COSFileID) == "" {
		return resp, fmt.Errorf("微信云上传凭证不完整: %+v", resp)
	}
	return resp, nil
}

func uploadFileToWechatStorage(uploadURL, cloudPath, filename, contentType string, fileBytes []byte, credential wechatUploadCredentialResp) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("key", strings.TrimSpace(cloudPath)); err != nil {
		return fmt.Errorf("写入上传 key 失败: %w", err)
	}
	if err := writer.WriteField("Signature", strings.TrimSpace(credential.Authorization)); err != nil {
		return fmt.Errorf("写入上传 Signature 失败: %w", err)
	}
	if strings.TrimSpace(credential.Token) != "" {
		if err := writer.WriteField("x-cos-security-token", strings.TrimSpace(credential.Token)); err != nil {
			return fmt.Errorf("写入上传 token 失败: %w", err)
		}
	}
	if err := writer.WriteField("x-cos-meta-fileid", strings.TrimSpace(credential.COSFileID)); err != nil {
		return fmt.Errorf("写入上传 fileid 失败: %w", err)
	}

	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if err := writer.WriteField("Content-Type", contentType); err != nil {
		return fmt.Errorf("写入上传 Content-Type 失败: %w", err)
	}

	partHeader := textproto.MIMEHeader{}
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filepath.Base(filename)))
	partHeader.Set("Content-Type", contentType)
	filePart, err := writer.CreatePart(partHeader)
	if err != nil {
		return fmt.Errorf("创建上传文件分片失败: %w", err)
	}
	if _, err := filePart.Write(fileBytes); err != nil {
		return fmt.Errorf("写入上传文件内容失败: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("关闭上传表单失败: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, strings.TrimSpace(uploadURL), &body)
	if err != nil {
		return fmt.Errorf("创建对象存储上传请求失败: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("上传对象存储失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("上传对象存储失败: HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func getWechatDownloadURL(accessToken, envID, fileID string, maxAge int) (string, error) {
	var resp wechatDownloadFileResp
	err := postWechatCloudAPI(accessToken, "/tcb/batchdownloadfile", map[string]interface{}{
		"env": strings.TrimSpace(envID),
		"file_list": []map[string]interface{}{
			{
				"fileid":  strings.TrimSpace(fileID),
				"max_age": maxAge,
			},
		},
	}, &resp)
	if err != nil {
		return "", err
	}
	if resp.ErrCode != 0 {
		return "", fmt.Errorf("微信云获取下载链接失败: errcode=%d errmsg=%s", resp.ErrCode, resp.ErrMsg)
	}
	if len(resp.FileList) == 0 {
		return "", fmt.Errorf("微信云获取下载链接失败: file_list 为空")
	}
	item := resp.FileList[0]
	if item.Status != 0 {
		return "", fmt.Errorf("微信云获取下载链接失败: status=%d errmsg=%s", item.Status, item.ErrMsg)
	}
	if strings.TrimSpace(item.DownloadURL) == "" {
		return "", fmt.Errorf("微信云获取下载链接失败: download_url 为空")
	}
	return strings.TrimSpace(item.DownloadURL), nil
}

func buildInlinePreviewURL(rawURL, contentType string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	query := parsedURL.Query()
	query.Set("response-content-disposition", "inline")
	contentType = strings.TrimSpace(contentType)
	if strings.HasPrefix(strings.ToLower(contentType), "image/") {
		query.Set("response-content-type", contentType)
	}
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String()
}

func uploadCourseImageByWechatCloudAPI(fileBytes []byte, fileName, contentType, cloudPath string, cfg storageConfig) (cloudBaseUploadResult, error) {
	accessToken, err := getWechatAccessToken()
	if err != nil {
		return cloudBaseUploadResult{}, err
	}

	credential, err := getWechatUploadCredential(accessToken, cfg.EnvID, cloudPath)
	if err != nil {
		return cloudBaseUploadResult{}, err
	}
	if err := uploadFileToWechatStorage(credential.URL, cloudPath, fileName, contentType, fileBytes, credential); err != nil {
		return cloudBaseUploadResult{}, err
	}

	downloadURL, err := getWechatDownloadURL(accessToken, cfg.EnvID, credential.FileID, 86400)
	if err != nil {
		return cloudBaseUploadResult{}, err
	}
	return cloudBaseUploadResult{
		FileID:      credential.FileID,
		TempFileURL: downloadURL,
		PreviewURL:  buildInlinePreviewURL(downloadURL, contentType),
	}, nil
}

func firstNonEmpty(keys ...string) string {
	for _, key := range keys {
		if val := strings.TrimSpace(os.Getenv(key)); val != "" {
			return val
		}
	}
	return ""
}

func firstNonEmptyValue(values ...string) string {
	for _, value := range values {
		if val := strings.TrimSpace(value); val != "" {
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
	cloudPath := fmt.Sprintf("images/%s/%d_%06d%s",
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
		"rawCloudEnvId": strings.TrimSpace(os.Getenv("CLOUD_ENV_ID")),
		"rawTCBEnv":     strings.TrimSpace(os.Getenv("TCB_ENV")),
		"rawTCBEnvID":   strings.TrimSpace(os.Getenv("TCB_ENVID")),
		"rawWXCloudEnv": strings.TrimSpace(os.Getenv("WX_CLOUD_ENV")),
		"rawEnvID":      strings.TrimSpace(os.Getenv("ENV_ID")),
		"bucketInfer":   inferEnvIDFromBucket(cfg.Bucket),
		"hasSecretID":   cfg.SecretID != "",
		"hasSecretKey":  cfg.SecretKey != "",
		"hasSessionTok": strings.TrimSpace(cfg.SessionToken) != "",
		"cloudPath":     cloudPath,
	})
	// #endregion

	result, err := uploadCourseImageByWechatCloudAPI(fileBytes, fileHeader.Filename, fileHeader.Header.Get("Content-Type"), cloudPath, cfg)
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
			"url":          firstNonEmptyValue(result.PreviewURL, result.TempFileURL),
			"preview_url":  firstNonEmptyValue(result.PreviewURL, result.TempFileURL),
			"download_url": result.TempFileURL,
			"key":          cloudPath,
			"fileID":       result.FileID,
			"source":       cfg.Source,
		},
	})
}
