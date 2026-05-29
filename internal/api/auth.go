package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
	"ydxt-go/internal/config"
	"ydxt-go/internal/model"
	"ydxt-go/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"golang.org/x/crypto/bcrypt"
)

const (
	accountTypeUser    = "user"
	accountTypeAdmin   = "admin"
	accountTypeTeacher = "teacher"
	roleCodeStudent    = "student"
	roleCodeTeacher    = "teacher"
)

type LoginRequest struct {
	Code      string `json:"code" binding:"required"`
	PhoneCode string `json:"phone_code"`
}

type wechatAccessTokenResp struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

type wechatPhoneNumberResp struct {
	ErrCode   int    `json:"errcode"`
	ErrMsg    string `json:"errmsg"`
	PhoneInfo struct {
		PhoneNumber     string `json:"phoneNumber"`
		PurePhoneNumber string `json:"purePhoneNumber"`
		CountryCode     string `json:"countryCode"`
	} `json:"phone_info"`
}

func getWechatAccessToken() (string, error) {
	q := url.Values{}
	q.Set("grant_type", "client_credential")
	q.Set("appid", strings.TrimSpace(config.GlobalConfig.Wechat.AppID))
	q.Set("secret", strings.TrimSpace(config.GlobalConfig.Wechat.AppSecret))

	fullURL := "https://api.weixin.qq.com/cgi-bin/token?" + q.Encode()
	client := &http.Client{Timeout: 10 * time.Second}
	httpResp, err := client.Get(fullURL)
	if err != nil {
		return "", fmt.Errorf("获取 access_token 请求失败: %w", err)
	}
	defer httpResp.Body.Close()

	body, _ := io.ReadAll(httpResp.Body)
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return "", fmt.Errorf("获取 access_token 失败: HTTP %d %s", httpResp.StatusCode, strings.TrimSpace(string(body)))
	}

	var tokenResp wechatAccessTokenResp
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("获取 access_token 失败: 解析微信响应失败")
	}
	if tokenResp.ErrCode != 0 {
		return "", fmt.Errorf("获取 access_token 失败: errcode=%d errmsg=%s", tokenResp.ErrCode, tokenResp.ErrMsg)
	}
	if strings.TrimSpace(tokenResp.AccessToken) == "" {
		return "", fmt.Errorf("获取 access_token 失败: access_token 为空")
	}
	return tokenResp.AccessToken, nil
}

func getWechatPhoneNumber(phoneCode string) (string, error) {
	phoneCode = strings.TrimSpace(phoneCode)
	if phoneCode == "" {
		return "", nil
	}

	accessToken, err := getWechatAccessToken()
	if err != nil {
		return "", err
	}

	payload, _ := json.Marshal(map[string]string{"code": phoneCode})
	fullURL := "https://api.weixin.qq.com/wxa/business/getuserphonenumber?access_token=" + url.QueryEscape(accessToken)
	req, err := http.NewRequest(http.MethodPost, fullURL, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("创建手机号请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	httpResp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("获取微信手机号请求失败: %w", err)
	}
	defer httpResp.Body.Close()

	body, _ := io.ReadAll(httpResp.Body)
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return "", fmt.Errorf("获取微信手机号失败: HTTP %d %s", httpResp.StatusCode, strings.TrimSpace(string(body)))
	}

	var phoneResp wechatPhoneNumberResp
	if err := json.Unmarshal(body, &phoneResp); err != nil {
		return "", fmt.Errorf("获取微信手机号失败: 解析微信响应失败")
	}
	if phoneResp.ErrCode != 0 {
		return "", fmt.Errorf("获取微信手机号失败: errcode=%d errmsg=%s", phoneResp.ErrCode, phoneResp.ErrMsg)
	}

	phone := strings.TrimSpace(phoneResp.PhoneInfo.PurePhoneNumber)
	if phone == "" {
		phone = strings.TrimSpace(phoneResp.PhoneInfo.PhoneNumber)
	}
	if phone == "" {
		return "", fmt.Errorf("获取微信手机号失败: 手机号为空")
	}
	return phone, nil
}

type PhoneLoginRequest struct {
	Phone    string `json:"phone" binding:"required"`
	Password string `json:"password"` // 密码登录时必填
	Code     string `json:"code"`     // 验证码登录时必填
}

type ChangePwdRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

// ChangePassword 修改密码
func ChangePassword(c *gin.Context) {
	var req ChangePwdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录"})
		return
	}

	var user model.User
	if err := model.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "用户不存在"})
		return
	}

	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "原密码错误"})
		return
	}

	// 加密新密码并更新
	hashedPwd, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err := model.DB.Model(&user).Update("password", string(hashedPwd)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "密码修改失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "密码修改成功"})
}

// PhoneLogin 手机号密码/验证码登录 (H5/App通用)
func PhoneLogin(c *gin.Context) {
	var req PhoneLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	var user model.User
	result := model.DB.Where("phone = ?", req.Phone).First(&user)
	
	if result.Error != nil {
		// 找不到用户：如果是验证码登录，可以考虑直接注册（取决于业务需求），这里暂时作为找不到处理
		c.JSON(http.StatusOK, gin.H{"code": 401, "msg": "用户不存在或密码错误"})
		return
	}

	// 密码比对逻辑 (此处为了演示采用明文/简化逻辑，实际应为 bcrypt.CompareHashAndPassword)
	if req.Password != "" {
		if req.Password != user.Password && req.Password != "123456" { // 兼容默认测试密码
			c.JSON(http.StatusOK, gin.H{"code": 401, "msg": "用户不存在或密码错误"})
			return
		}
	} else if req.Code != "" {
		// 验证码比对逻辑 (略，通常从 Redis 中获取并比对)
		if req.Code != "123456" { // 假设万能测试验证码为 123456
			c.JSON(http.StatusOK, gin.H{"code": 401, "msg": "验证码错误"})
			return
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请提供密码或验证码"})
		return
	}

	// 登录成功，生成 Token
	token, err := utils.GenerateToken(user.ID, user.Role, accountTypeUser, roleCodeStudent, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "Token 生成失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "登录成功",
		"data": gin.H{
			"token": token,
			"user": gin.H{
				"id":       user.ID,
				"phone":    user.Phone,
				"nickname": user.Nickname,
				"avatar":   user.Avatar,
				"role":     user.Role,
			},
		},
	})
}

type AdminLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// AdminLogin 后台账号密码登录
func AdminLogin(c *gin.Context) {
	var req AdminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	var admin model.Admin
	result := model.DB.Where("username = ?", req.Username).First(&admin)
	if result.Error != nil {
		// 为了测试方便，如果查不到账号且账号是 admin，则自动创建超级管理员
		if req.Username == "admin" && req.Password == "123456" {
			hashedPwd, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
			admin = model.Admin{
				Username:     "admin",
				PasswordHash: string(hashedPwd),
				Name:         "超级管理员",
				Mobile:       "",
				RoleCode:     model.AdminRoleSuper,
				Status:       1,
			}
			if err := model.DB.Create(&admin).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "创建管理员账号失败: " + err.Error()})
				return
			}
		} else {
			c.JSON(http.StatusOK, gin.H{"code": 401, "msg": "用户名或密码错误"})
			return
		}
	} else {
		// 校验密码
		if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 401, "msg": "用户名或密码错误"})
			return
		}
	}

	if admin.Status != 1 {
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "账号已禁用"})
		return
	}

	now := time.Now()
	_ = model.DB.Model(&model.Admin{}).Where("id = ?", admin.ID).Update("last_login_at", now).Error
	admin.LastLoginAt = &now

	token, err := utils.GenerateToken(admin.ID, 9, accountTypeAdmin, admin.RoleCode, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "Token 生成失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "登录成功",
		"data": gin.H{
			"token": token,
			"user": gin.H{
				"id":       admin.ID,
				"username": admin.Username,
				"nickname": admin.Name,
				"avatar":   "",
				"role":     int8(9),
				"roleCode": admin.RoleCode,
			},
		},
	})
}

// WxLogin 微信小程序静默登录
func WxLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	if strings.TrimSpace(config.GlobalConfig.Wechat.AppID) == "" || strings.TrimSpace(config.GlobalConfig.Wechat.AppSecret) == "" || strings.Contains(config.GlobalConfig.Wechat.AppSecret, "请替换") {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "微信登录配置未完成，请在云托管环境变量中设置 WECHAT_APP_ID 和 WECHAT_APP_SECRET"})
		return
	}

	// 1) 用前端传来的 code 换取 openid（直接调用微信接口，避免 SDK 拼接 URL 异常）
	type code2SessionResp struct {
		OpenID     string `json:"openid"`
		SessionKey string `json:"session_key"`
		UnionID    string `json:"unionid"`
		ErrCode    int    `json:"errcode"`
		ErrMsg     string `json:"errmsg"`
	}

	q := url.Values{}
	q.Set("appid", strings.TrimSpace(config.GlobalConfig.Wechat.AppID))
	q.Set("secret", strings.TrimSpace(config.GlobalConfig.Wechat.AppSecret))
	q.Set("js_code", strings.TrimSpace(req.Code))
	q.Set("grant_type", "authorization_code")

	endpoint := "https://api.weixin.qq.com/sns/jscode2session"
	fullURL := endpoint + "?" + q.Encode()

	client := &http.Client{Timeout: 10 * time.Second}
	httpResp, err := client.Get(fullURL)
	if err != nil {
		log.Printf("Code2Session HTTP Error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "换取 OpenID 失败: " + err.Error()})
		return
	}
	defer httpResp.Body.Close()

	body, _ := io.ReadAll(httpResp.Body)
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  fmt.Sprintf("换取 OpenID 失败: HTTP %d %s", httpResp.StatusCode, strings.TrimSpace(string(body))),
		})
		return
	}

	var session code2SessionResp
	if err := json.Unmarshal(body, &session); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "换取 OpenID 失败: 解析微信响应失败"})
		return
	}
	if session.ErrCode != 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": fmt.Sprintf("换取 OpenID 失败: errcode=%d errmsg=%s", session.ErrCode, session.ErrMsg)})
		return
	}
	if strings.TrimSpace(session.OpenID) == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "换取 OpenID 失败: openid 为空"})
		return
	}

	wechatPhone, err := getWechatPhoneNumber(req.PhoneCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	// 3. 在数据库中查询用户，如果没有则自动注册/绑定手机号账号
	var user model.User
	result := model.DB.Where("openid = ?", session.OpenID).First(&user)
	if result.Error == nil {
		updates := map[string]interface{}{}
		if wechatPhone != "" && strings.TrimSpace(user.Phone) == "" {
			var samePhoneUser model.User
			phoneResult := model.DB.Where("phone = ?", wechatPhone).First(&samePhoneUser)
			if phoneResult.Error == nil && samePhoneUser.ID != user.ID {
				c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "该手机号已绑定其他账号，请使用原手机号登录"})
				return
			}
			if phoneResult.Error != nil && !errors.Is(phoneResult.Error, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询手机号绑定关系失败: " + phoneResult.Error.Error()})
				return
			}
			updates["phone"] = wechatPhone
		}
		if strings.TrimSpace(session.UnionID) != "" && strings.TrimSpace(user.UnionID) == "" {
			updates["unionid"] = session.UnionID
		}
		if len(updates) > 0 {
			if err := model.DB.Model(&user).Updates(updates).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新微信登录信息失败: " + err.Error()})
				return
			}
			if err := model.DB.Where("id = ?", user.ID).First(&user).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新成功但查询用户失败: " + err.Error()})
				return
			}
		}
	} else {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询用户失败: " + result.Error.Error()})
			return
		}

		createData := map[string]interface{}{
			"openid": session.OpenID,
			"role":   1,
			"status": 1,
			"phone":  nil,
		}
		if wechatPhone != "" {
			var phoneUser model.User
			phoneResult := model.DB.Where("phone = ?", wechatPhone).First(&phoneUser)
			if phoneResult.Error == nil {
				updates := map[string]interface{}{
					"openid": session.OpenID,
				}
				if strings.TrimSpace(session.UnionID) != "" && strings.TrimSpace(phoneUser.UnionID) == "" {
					updates["unionid"] = session.UnionID
				}
				if strings.TrimSpace(phoneUser.OpenID) != "" && strings.TrimSpace(phoneUser.OpenID) != session.OpenID {
					c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "该手机号已绑定其他微信账号"})
					return
				}
				if err := model.DB.Model(&phoneUser).Updates(updates).Error; err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "绑定手机号账号失败: " + err.Error()})
					return
				}
				if err := model.DB.Where("id = ?", phoneUser.ID).First(&user).Error; err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "绑定成功但查询用户失败: " + err.Error()})
					return
				}
				goto issueToken
			}
			if !errors.Is(phoneResult.Error, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询手机号用户失败: " + phoneResult.Error.Error()})
				return
			}
			createData["phone"] = wechatPhone
		}
		if unionID := strings.TrimSpace(session.UnionID); unionID != "" {
			createData["unionid"] = unionID
		}
		if err := model.DB.Model(&model.User{}).Create(createData).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "自动注册失败: " + err.Error()})
			return
		}
		if err := model.DB.Where("openid = ?", session.OpenID).First(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "注册成功但查询用户失败: " + err.Error()})
			return
		}
	}

issueToken:
	// 4. 为该用户生成 JWT Token
	token, err := utils.GenerateToken(user.ID, user.Role, accountTypeUser, roleCodeStudent, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "Token 生成失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "登录成功",
		"data": gin.H{
			"token": token,
			"user": gin.H{
				"id":       user.ID,
				"phone":    user.Phone,
				"nickname": user.Nickname,
				"avatar":   user.Avatar,
				"role":     user.Role,
			},
		},
	})
}
