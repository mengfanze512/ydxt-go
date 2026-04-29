package api

import (
	"log"
	"net/http"
	"yuedi_edu/internal/config"
	"yuedi_edu/internal/model"
	"yuedi_edu/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/silenceper/wechat/v2"
	"github.com/silenceper/wechat/v2/cache"
	"github.com/silenceper/wechat/v2/miniprogram"
	miniConfig "github.com/silenceper/wechat/v2/miniprogram/config"
)

type LoginRequest struct {
	Code string `json:"code"`
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

	var user model.User
	// 这里用 Phone 字段暂代 Username
	result := model.DB.Where("phone = ?", req.Username).First(&user)
	if result.Error != nil {
		// 为了测试方便，如果查不到账号且账号是 admin，则自动创建超级管理员
		if req.Username == "admin" && req.Password == "123456" {
			user = model.User{
				Phone:    "admin",
				Password: "admin_password_hash", // 这里实际应该存 bcrypt hash
				Role:     9,
				Status:   1,
				Nickname: "超级管理员",
			}
			model.DB.Create(&user)
		} else {
			c.JSON(http.StatusOK, gin.H{"code": 401, "msg": "用户名或密码错误"})
			return
		}
	}

	// 实际应用应该用 bcrypt 校验，这里为了快速测试简化为明文比对或模拟校验
	// if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil { ... }
	if req.Username != "admin" && req.Password != "123456" {
		c.JSON(http.StatusOK, gin.H{"code": 401, "msg": "用户名或密码错误"})
		return
	}

	// 校验权限 (只有 role=9 管理员能登录后台)
	if user.Role != 9 {
		c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "无权限登录管理后台"})
		return
	}

	token, err := utils.GenerateToken(user.ID, user.Role)
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
				"username": user.Phone,
				"nickname": user.Nickname,
				"avatar":   user.Avatar,
				"role":     user.Role,
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

	// 1. 初始化微信 SDK 配置
	wc := wechat.NewWechat()
	memory := cache.NewMemory()
	cfg := &miniConfig.Config{
		AppID:     config.GlobalConfig.Wechat.AppID,
		AppSecret: config.GlobalConfig.Wechat.AppSecret,
		Cache:     memory,
	}
	mini := wc.GetMiniProgram(cfg)

	// 2. 用前端传来的 code 换取 openid
	auth := mini.GetAuth()
	session, err := auth.Code2Session(req.Code)
	if err != nil {
		log.Printf("Code2Session Error: %v\n", err)
		// 如果本地测试没配 AppSecret，这里可以做个 mock
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "换取 OpenID 失败，请检查配置"})
		return
	}

	// 3. 在数据库中查询用户，如果没有则自动注册
	var user model.User
	result := model.DB.Where("openid = ?", session.OpenID).First(&user)
	if result.Error != nil {
		// 查不到，自动注册新用户 (默认身份是学生 role=1)
		user = model.User{
			OpenID: session.OpenID,
			Role:   1, 
			Status: 1,
		}
		if err := model.DB.Create(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "自动注册失败"})
			return
		}
	}

	// 4. 为该用户生成 JWT Token
	token, err := utils.GenerateToken(user.ID, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "Token 生成失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "登录成功",
		"data": gin.H{
			"token":   token,
			"user_id": user.ID,
			"role":    user.Role,
		},
	})
}
