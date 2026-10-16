package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"ydxt-go/internal/config"

	"github.com/gin-gonic/gin"
)

var wechatLoginClient = &http.Client{Timeout: 10 * time.Second}

type LoginRequest struct {
	Code      string `json:"code" binding:"required"`
	PhoneCode string `json:"phone_code"`
}

func wechatJSON(req *http.Request, result interface{}) error {
	response, err := wechatLoginClient.Do(req)
	if err != nil {
		return fmt.Errorf("微信接口请求失败: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("微信接口返回 HTTP %d", response.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(result); err != nil {
		return fmt.Errorf("解析微信响应失败: %w", err)
	}
	return nil
}

func getWechatAccessToken() (string, error) {
	params := url.Values{"grant_type": {"client_credential"}, "appid": {strings.TrimSpace(config.GlobalConfig.Wechat.AppID)}, "secret": {strings.TrimSpace(config.GlobalConfig.Wechat.AppSecret)}}
	req, err := http.NewRequest("GET", "https://api.weixin.qq.com/cgi-bin/token?"+params.Encode(), nil)
	if err != nil {
		return "", err
	}
	var result struct {
		AccessToken string `json:"access_token"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := wechatJSON(req, &result); err != nil {
		return "", err
	}
	if result.ErrCode != 0 || result.AccessToken == "" {
		return "", fmt.Errorf("获取微信 access_token 失败: errcode=%d errmsg=%s", result.ErrCode, result.ErrMsg)
	}
	return result.AccessToken, nil
}

func getWechatPhoneNumber(code string) (string, error) {
	if strings.TrimSpace(code) == "" {
		return "", nil
	}
	token, err := getWechatAccessToken()
	if err != nil {
		return "", err
	}
	body, _ := json.Marshal(gin.H{"code": code})
	req, err := http.NewRequest("POST", "https://api.weixin.qq.com/wxa/business/getuserphonenumber?access_token="+url.QueryEscape(token), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	var result struct {
		ErrCode   int    `json:"errcode"`
		ErrMsg    string `json:"errmsg"`
		PhoneInfo struct {
			PhoneNumber     string `json:"phoneNumber"`
			PurePhoneNumber string `json:"purePhoneNumber"`
		} `json:"phone_info"`
	}
	if err := wechatJSON(req, &result); err != nil {
		return "", err
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("获取微信手机号失败: errcode=%d errmsg=%s", result.ErrCode, result.ErrMsg)
	}
	phone := strings.TrimSpace(result.PhoneInfo.PurePhoneNumber)
	if phone == "" {
		phone = strings.TrimSpace(result.PhoneInfo.PhoneNumber)
	}
	if phone == "" {
		return "", fmt.Errorf("微信响应没有有效手机号")
	}
	return phone, nil
}

func WxLogin(c *gin.Context) {
	if !deliveryDB(c) {
		return
	}
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "微信登录参数错误: " + err.Error()})
		return
	}
	cfg := config.GlobalConfig.Wechat
	if strings.TrimSpace(cfg.AppID) == "" || strings.TrimSpace(cfg.AppSecret) == "" || strings.Contains(cfg.AppSecret, "请替换") {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "微信登录配置未完成，请配置 WECHAT_APP_ID 和 WECHAT_APP_SECRET"})
		return
	}
	params := url.Values{"appid": {cfg.AppID}, "secret": {cfg.AppSecret}, "js_code": {req.Code}, "grant_type": {"authorization_code"}}
	request, _ := http.NewRequest("GET", "https://api.weixin.qq.com/sns/jscode2session?"+params.Encode(), nil)
	var session struct {
		OpenID  string `json:"openid"`
		UnionID string `json:"unionid"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := wechatJSON(request, &session); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"code": 502, "msg": "换取 OpenID 失败: " + err.Error()})
		return
	}
	if session.ErrCode != 0 || session.OpenID == "" {
		c.JSON(http.StatusBadGateway, gin.H{"code": 502, "msg": fmt.Sprintf("换取 OpenID 失败: errcode=%d errmsg=%s", session.ErrCode, session.ErrMsg)})
		return
	}
	phone, err := getWechatPhoneNumber(req.PhoneCode)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"code": 502, "msg": err.Error()})
		return
	}
	if phone != "" {
		staff, err := resolveWechatStaff(phone)
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "识别微信账号身份失败: " + err.Error()})
			return
		}
		if staff != nil {
			issueAccountLogin(c, staff.AccountType, staff.ID)
			return
		}
	}
	id, err := bindWechatStudent(session.OpenID, session.UnionID, phone)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "绑定学员账号失败: " + err.Error()})
		return
	}
	issueAccountLogin(c, accountTypeUser, id)
}
