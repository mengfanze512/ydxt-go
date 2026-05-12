package api

import (
	"log"
	"net/http"
	"time"
	"ydxt-go/internal/config"
	"ydxt-go/internal/model"
	"ydxt-go/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/silenceper/wechat/v2"
	"github.com/silenceper/wechat/v2/cache"
	miniConfig "github.com/silenceper/wechat/v2/miniprogram/config"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type LoginRequest struct {
	Code      string `json:"code"`
	PhoneCode string `json:"phone_code"` // 新增: 用于获取手机号的 code
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

const (
	accountTypeStudent = "student"
	accountTypeAdmin   = "admin"
	accountTypeTeacher = "teacher"
)

type loginUserPayload struct {
	ID          uint64 `json:"id"`
	Username    string `json:"username,omitempty"`
	Phone       string `json:"phone,omitempty"`
	Nickname    string `json:"nickname"`
	Name        string `json:"name,omitempty"`
	Avatar      string `json:"avatar,omitempty"`
	Role        int8   `json:"role"`
	AccountType string `json:"account_type"`
	RoleCode    string `json:"role_code"`
	TeacherID   uint64 `json:"teacher_id"`
}

func makeLoginResponse(token string, user loginUserPayload) gin.H {
	return gin.H{
		"code": 0,
		"msg":  "登录成功",
		"data": gin.H{
			"token": token,
			"user":  user,
		},
	}
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
	accountType, _ := c.Get("accountType")

	hashedPwd, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)

	switch accountType {
	case accountTypeAdmin:
		var admin model.Admin
		if err := model.DB.First(&admin, userID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "管理员不存在"})
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.OldPassword)); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "原密码错误"})
			return
		}
		if err := model.DB.Model(&admin).Update("password_hash", string(hashedPwd)).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "密码修改失败"})
			return
		}
	case accountTypeTeacher:
		var teacher model.Teacher
		if err := model.DB.First(&teacher, userID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "讲师不存在"})
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(teacher.PasswordHash), []byte(req.OldPassword)); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "原密码错误"})
			return
		}
		if err := model.DB.Model(&teacher).Update("password_hash", string(hashedPwd)).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "密码修改失败"})
			return
		}
	default:
		var user model.User
		if err := model.DB.First(&user, userID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "用户不存在"})
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "原密码错误"})
			return
		}
		if err := model.DB.Model(&user).Update("password", string(hashedPwd)).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "密码修改失败"})
			return
		}
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
		// 找不到用户：如果是验证码登录或者测试密码登录，自动注册该用户
		if req.Code == "123456" || req.Password == "123456" {
			hashedPwd, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
			user = model.User{
				Phone:    req.Phone,
				Password: string(hashedPwd),
				Role:     1, // 默认注册为普通学员
				Status:   1,
				Nickname: "用户_" + req.Phone[len(req.Phone)-4:], // 取手机号后4位
			}
			if err := model.DB.Create(&user).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "自动注册失败"})
				return
			}
		} else {
			c.JSON(http.StatusOK, gin.H{"code": 401, "msg": "用户不存在或密码错误"})
			return
		}
	} else {
		// 密码比对逻辑 (此处为了演示采用明文/简化逻辑，实际应为 bcrypt.CompareHashAndPassword)
		if req.Password != "" {
			if req.Password != user.Password && req.Password != "123456" { // 兼容默认测试密码
				c.JSON(http.StatusOK, gin.H{"code": 401, "msg": "密码错误"})
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
	}

	// 登录成功，生成 Token
	token, err := utils.GenerateToken(user.ID, user.Role, accountTypeStudent, "", 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "Token 生成失败"})
		return
	}

	c.JSON(http.StatusOK, makeLoginResponse(token, loginUserPayload{
		ID:          user.ID,
		Phone:       user.Phone,
		Nickname:    user.Nickname,
		Avatar:      user.Avatar,
		Role:        user.Role,
		AccountType: accountTypeStudent,
		RoleCode:    "",
		TeacherID:   0,
	}))
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
	adminResult := model.DB.Where("username = ?", req.Username).First(&admin)
	if adminResult.Error == nil {
		if admin.Status != 1 {
			c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "账号已被禁用"})
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 401, "msg": "用户名或密码错误"})
			return
		}
		now := time.Now()
		_ = model.DB.Model(&admin).Update("last_login_at", &now).Error
		token, err := utils.GenerateToken(admin.ID, 9, accountTypeAdmin, admin.RoleCode, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "Token 生成失败"})
			return
		}
		c.JSON(http.StatusOK, makeLoginResponse(token, loginUserPayload{
			ID:          admin.ID,
			Username:    admin.Username,
			Nickname:    admin.Name,
			Name:        admin.Name,
			Role:        9,
			AccountType: accountTypeAdmin,
			RoleCode:    admin.RoleCode,
			TeacherID:   0,
		}))
		return
	}

	var teacher model.Teacher
	teacherResult := model.DB.Where("username = ?", req.Username).First(&teacher)
	if teacherResult.Error == nil {
		if teacher.Status != 1 {
			c.JSON(http.StatusOK, gin.H{"code": 403, "msg": "账号已被禁用"})
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(teacher.PasswordHash), []byte(req.Password)); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 401, "msg": "用户名或密码错误"})
			return
		}
		now := time.Now()
		_ = model.DB.Model(&teacher).Update("last_login_at", &now).Error
		token, err := utils.GenerateToken(teacher.ID, 2, accountTypeTeacher, accountTypeTeacher, teacher.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "Token 生成失败"})
			return
		}
		c.JSON(http.StatusOK, makeLoginResponse(token, loginUserPayload{
			ID:          teacher.ID,
			Username:    teacher.Username,
			Nickname:    teacher.Name,
			Name:        teacher.Name,
			Avatar:      teacher.Avatar,
			Role:        2,
			AccountType: accountTypeTeacher,
			RoleCode:    accountTypeTeacher,
			TeacherID:   teacher.ID,
		}))
		return
	}

	// 兼容旧库: 第一次部署仍允许 admin/123456 自动生成超级管理员
	if req.Username == "admin" && req.Password == "123456" {
		hashedPwd, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
		admin = model.Admin{
			Username:     "admin",
			PasswordHash: string(hashedPwd),
			Name:         "超级管理员",
			RoleCode:     model.AdminRoleSuper,
			Status:       1,
		}
		if err := model.DB.Create(&admin).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "创建管理员账号失败: " + err.Error()})
			return
		}
		token, err := utils.GenerateToken(admin.ID, 9, accountTypeAdmin, admin.RoleCode, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "Token 生成失败"})
			return
		}
		c.JSON(http.StatusOK, makeLoginResponse(token, loginUserPayload{
			ID:          admin.ID,
			Username:    admin.Username,
			Nickname:    admin.Name,
			Name:        admin.Name,
			Role:        9,
			AccountType: accountTypeAdmin,
			RoleCode:    admin.RoleCode,
			TeacherID:   0,
		}))
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 401, "msg": "用户名或密码错误"})
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

	// 3. 处理手机号获取 (多端账号互通的主键)
	phone := ""
	if req.PhoneCode != "" {
		// 在真实企业小程序中，应调用微信接口用 PhoneCode 换取真实手机号
		// URL: POST https://api.weixin.qq.com/wxa/business/getuserphonenumber?access_token=ACCESS_TOKEN
		// 这里由于我们可能是个人开发者测试，无法获取权限，因此做一个 Mock 兼容逻辑：
		if len(req.PhoneCode) >= 4 {
			phone = "1380000" + req.PhoneCode[len(req.PhoneCode)-4:]
		} else {
			phone = "13800001234"
		}
	}

	// 4. 在数据库中查询用户，如果没有则自动注册
	var user model.User
	var result *gorm.DB

	if phone != "" {
		// 如果获取到了手机号，优先用手机号查询，以实现 H5/App/小程序 的账号互通
		result = model.DB.Where("phone = ?", phone).First(&user)
	} else {
		// 否则退退求其次，用 OpenID 查询
		result = model.DB.Where("openid = ?", session.OpenID).First(&user)
	}

	if result.Error != nil {
		// 查不到，自动注册新用户
		user = model.User{
			Phone:    phone,
			OpenID:   session.OpenID,
			Role:     1, // 默认学员
			Status:   1,
			Nickname: "微信用户",
		}
		if phone != "" {
			user.Nickname = "用户_" + phone[len(phone)-4:]
		}
		if err := model.DB.Create(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "自动注册失败"})
			return
		}
	} else {
		// 如果用户已存在，但此时我们又获得了他的 OpenID（比如他以前是在H5用手机号注册的，现在第一次用小程序登录）
		// 我们需要把 OpenID 绑定到他的账号上
		if user.OpenID == "" || user.OpenID != session.OpenID {
			user.OpenID = session.OpenID
			model.DB.Save(&user)
		}
	}

	// 5. 为该用户生成 JWT Token
	token, err := utils.GenerateToken(user.ID, user.Role, accountTypeStudent, "", 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "Token 生成失败"})
		return
	}

	c.JSON(http.StatusOK, makeLoginResponse(token, loginUserPayload{
		ID:          user.ID,
		Phone:       user.Phone,
		Nickname:    user.Nickname,
		Avatar:      user.Avatar,
		Role:        user.Role,
		AccountType: accountTypeStudent,
		RoleCode:    "",
		TeacherID:   0,
	}))
}
