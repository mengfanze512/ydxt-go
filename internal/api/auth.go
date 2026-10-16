package api

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"ydxt-go/internal/model"
	"ydxt-go/internal/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	accountTypeUser    = "user"
	accountTypeAdmin   = "admin"
	accountTypeTeacher = "teacher"
	roleCodeStudent    = "student"
	roleCodeTeacher    = "teacher"
)

type PhoneLoginRequest struct {
	Account  string `json:"account"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
	Code     string `json:"code"`
}

type AdminLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type ChangePwdRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

type loginCandidate struct {
	AccountType string
	ID          uint64
	Password    string
}

func checkPassword(input, stored string) bool {
	if input == "" || stored == "" {
		return false
	}
	if strings.HasPrefix(stored, "$2") {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(input)) == nil
	}
	return subtle.ConstantTimeCompare([]byte(input), []byte(stored)) == 1
}

// A matching legacy plaintext password is upgraded only after successful verification.
func upgradePassword(candidate loginCandidate, password string) error {
	if strings.HasPrefix(candidate.Password, "$2") {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	table, column := accountPasswordColumn(candidate.AccountType)
	result := model.DB.Table(table).Where("id = ? AND "+column+" = ?", candidate.ID, candidate.Password).
		Update(column, string(hash))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("账号密码已变更，请重新登录")
	}
	return nil
}

func accountPasswordColumn(accountType string) (string, string) {
	switch accountType {
	case accountTypeAdmin:
		return "admins", "password_hash"
	case accountTypeTeacher:
		return "teachers", "password_hash"
	default:
		return "users", "password"
	}
}

func findLoginCandidates(identifier string, staffOnly bool) ([]loginCandidate, error) {
	items := []loginCandidate{}
	var admins []model.Admin
	if err := model.DB.Where("username = ? OR (mobile <> '' AND mobile = ?)", identifier, identifier).Find(&admins).Error; err != nil {
		return nil, fmt.Errorf("查询管理员账号失败: %w", err)
	}
	for _, admin := range admins {
		items = append(items, loginCandidate{accountTypeAdmin, admin.ID, admin.PasswordHash})
	}
	var teachers []model.Teacher
	if err := model.DB.Where("username = ? OR (mobile <> '' AND mobile = ?)", identifier, identifier).Find(&teachers).Error; err != nil {
		return nil, fmt.Errorf("查询讲师账号失败: %w", err)
	}
	for _, teacher := range teachers {
		items = append(items, loginCandidate{accountTypeTeacher, teacher.ID, teacher.PasswordHash})
	}
	if !staffOnly {
		var users []model.User
		if err := model.DB.Where("phone = ?", identifier).Find(&users).Error; err != nil {
			return nil, fmt.Errorf("查询学员账号失败: %w", err)
		}
		for _, user := range users {
			items = append(items, loginCandidate{accountTypeUser, user.ID, user.Password})
		}
	}
	return items, nil
}

func issueAccountLogin(c *gin.Context, accountType string, id uint64) {
	identity, err := utils.LoadAccount(accountType, id)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": err.Error()})
		return
	}
	token, err := utils.GenerateToken(identity.ID, identity.Role, identity.AccountType, identity.RoleCode, identity.TeacherID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "生成登录凭证失败: " + err.Error()})
		return
	}
	if accountType != accountTypeUser {
		table, _ := accountPasswordColumn(accountType)
		if err := model.DB.Table(table).Where("id = ?", id).Update("last_login_at", time.Now()).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新登录记录失败: " + err.Error()})
			return
		}
	}
	home := "/pages/index/index"
	if identity.RoleCode == model.AdminRoleSuper {
		home = "/pages/operation/dashboard"
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "登录成功", "data": gin.H{
		"token": token, "user": identity, "home": home,
	}})
}

func passwordLogin(c *gin.Context, identifier, password string, staffOnly bool) {
	if !deliveryDB(c) {
		return
	}
	if identifier == "" || password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请输入账号和密码"})
		return
	}
	if len(password) > 72 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "密码不能超过 72 字节"})
		return
	}
	candidates, err := findLoginCandidates(identifier, staffOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	matches := []loginCandidate{}
	for _, candidate := range candidates {
		if checkPassword(password, candidate.Password) {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "账号或密码错误"})
		return
	}
	if len(matches) != 1 {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "该登录标识和密码匹配多个账号，请使用唯一用户名或联系管理员调整账号"})
		return
	}
	candidate := matches[0]
	if _, err := utils.LoadAccount(candidate.AccountType, candidate.ID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": err.Error()})
		return
	}
	if err := upgradePassword(candidate, password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "升级账号密码存储失败: " + err.Error()})
		return
	}
	issueAccountLogin(c, candidate.AccountType, candidate.ID)
}

func PhoneLogin(c *gin.Context) {
	var req PhoneLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "登录参数错误: " + err.Error()})
		return
	}
	if req.Code != "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "短信验证码登录尚未接入，请使用账号密码或微信授权登录"})
		return
	}
	identifier := strings.TrimSpace(req.Account)
	if identifier == "" {
		identifier = strings.TrimSpace(req.Phone)
	}
	passwordLogin(c, identifier, req.Password, false)
}

func AdminLogin(c *gin.Context) {
	var req AdminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "登录参数错误: " + err.Error()})
		return
	}
	passwordLogin(c, strings.TrimSpace(req.Username), req.Password, true)
}

func GetAccountProfile(c *gin.Context) {
	identity, ok := c.Get("accountIdentity")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "登录身份缺失，请重新登录"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": identity})
}

func ChangePassword(c *gin.Context) {
	var req ChangePwdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "密码参数错误: " + err.Error()})
		return
	}
	value, ok := c.Get("accountIdentity")
	identity, valid := value.(*utils.AccountIdentity)
	if !ok || !valid {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "请重新登录后修改密码"})
		return
	}
	if !checkPassword(req.OldPassword, identity.Password) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "原密码错误"})
		return
	}
	if len(req.NewPassword) < 8 || len(req.NewPassword) > 72 || strings.TrimSpace(req.NewPassword) == "" || req.NewPassword == req.OldPassword {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "新密码需为 8 至 72 字节，且不能与原密码相同"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "密码加密失败: " + err.Error()})
		return
	}
	table, column := accountPasswordColumn(identity.AccountType)
	result := model.DB.Table(table).Where("id = ? AND "+column+" = ?", identity.ID, identity.Password).Update(column, string(hash))
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "修改密码失败: " + result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "密码已被修改，请重新登录"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "密码修改成功，请重新登录"})
}

func resolveWechatStaff(phone string) (*loginCandidate, error) {
	var admins []model.Admin
	if err := model.DB.Where("mobile = ?", phone).Find(&admins).Error; err != nil {
		return nil, err
	}
	var teachers []model.Teacher
	if err := model.DB.Where("mobile = ?", phone).Find(&teachers).Error; err != nil {
		return nil, err
	}
	if len(admins)+len(teachers) > 1 {
		return nil, errors.New("该手机号关联多个管理或讲师账号，请使用唯一用户名和密码登录")
	}
	if len(admins) == 1 {
		return &loginCandidate{accountTypeAdmin, admins[0].ID, admins[0].PasswordHash}, nil
	}
	if len(teachers) == 1 {
		return &loginCandidate{accountTypeTeacher, teachers[0].ID, teachers[0].PasswordHash}, nil
	}
	return nil, nil
}

func bindWechatStudent(openID, unionID, phone string) (uint64, error) {
	var id uint64
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var user model.User
		err := tx.Where("openid = ?", openID).First(&user).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if phone != "" {
			var phoneUser model.User
			phoneErr := tx.Where("phone = ?", phone).First(&phoneUser).Error
			if phoneErr != nil && !errors.Is(phoneErr, gorm.ErrRecordNotFound) {
				return phoneErr
			}
			if phoneErr == nil {
				if (user.ID != 0 && user.ID != phoneUser.ID) || (phoneUser.OpenID != "" && phoneUser.OpenID != openID) {
					return errors.New("该手机号已绑定其他账号，请使用原账号登录")
				}
				user = phoneUser
			}
			if user.Phone != "" && user.Phone != phone {
				return errors.New("微信手机号与当前账号绑定手机号不一致，请使用原账号登录")
			}
		}
		if user.ID == 0 {
			values := map[string]interface{}{"openid": openID, "role": 1, "status": 1, "phone": nil}
			if phone != "" {
				values["phone"] = phone
			}
			if unionID != "" {
				values["unionid"] = unionID
			}
			if err := tx.Model(&model.User{}).Create(values).Error; err != nil {
				return err
			}
			if err := tx.Where("openid = ?", openID).First(&user).Error; err != nil {
				return err
			}
		} else {
			if user.Status != 1 || user.IsDeleted != 0 || user.Role != 1 {
				return errors.New("学员账号已停用或角色配置异常，请联系管理员")
			}
			values := map[string]interface{}{"openid": openID}
			if phone != "" {
				values["phone"] = phone
			}
			if unionID != "" && user.UnionID == "" {
				values["unionid"] = unionID
			}
			if err := tx.Model(&user).Updates(values).Error; err != nil {
				return err
			}
		}
		id = user.ID
		return nil
	})
	return id, err
}
