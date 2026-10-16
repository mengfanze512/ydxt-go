package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"ydxt-go/internal/config"
	"ydxt-go/internal/model"
)

type AccountIdentity struct {
	ID          uint64 `json:"id"`
	AccountType string `json:"account_type"`
	Role        int8   `json:"role"`
	RoleCode    string `json:"role_code"`
	TeacherID   uint64 `json:"teacher_id"`
	Username    string `json:"username"`
	Phone       string `json:"phone"`
	Nickname    string `json:"nickname"`
	Avatar      string `json:"avatar"`
	Password    string `json:"-"`
}

func LoadAccount(accountType string, id uint64) (*AccountIdentity, error) {
	if model.DB == nil {
		return nil, errors.New("数据库未连接")
	}
	identity := &AccountIdentity{ID: id, AccountType: accountType}
	switch accountType {
	case "user":
		var user model.User
		if err := model.DB.First(&user, id).Error; err != nil {
			return nil, err
		}
		if user.Status != 1 || user.IsDeleted != 0 {
			return nil, errors.New("学员账号已禁用或删除")
		}
		if user.Role != 1 {
			return nil, errors.New("旧角色账号需要在讲师或管理员账号表中配置，请联系管理员")
		}
		identity.Role, identity.RoleCode = 1, "student"
		identity.Phone, identity.Nickname, identity.Avatar, identity.Password = user.Phone, user.Nickname, user.Avatar, user.Password
	case "teacher":
		var teacher model.Teacher
		if err := model.DB.First(&teacher, id).Error; err != nil {
			return nil, err
		}
		if teacher.Status != 1 {
			return nil, errors.New("讲师账号已禁用")
		}
		identity.Role, identity.RoleCode, identity.TeacherID = 2, "teacher", teacher.ID
		identity.Username, identity.Phone, identity.Nickname, identity.Avatar, identity.Password = teacher.Username, teacher.Mobile, teacher.Name, teacher.Avatar, teacher.PasswordHash
	case "admin":
		var admin model.Admin
		if err := model.DB.First(&admin, id).Error; err != nil {
			return nil, err
		}
		if admin.Status != 1 {
			return nil, errors.New("管理员账号已禁用")
		}
		if admin.RoleCode != model.AdminRoleSuper && admin.RoleCode != model.AdminRoleNormal {
			return nil, errors.New("管理员角色配置无效，请联系超级管理员")
		}
		identity.Role, identity.RoleCode = 9, admin.RoleCode
		identity.Username, identity.Phone, identity.Nickname, identity.Password = admin.Username, admin.Mobile, admin.Name, admin.PasswordHash
	default:
		return nil, errors.New("账号身份无效，请重新登录")
	}
	return identity, nil
}

// Sign a credential fingerprint rather than putting a password hash into JWT.
func accountCredentialVersion(identity *AccountIdentity) string {
	mac := hmac.New(sha256.New, []byte(config.GlobalConfig.JWT.Secret))
	fmt.Fprintf(mac, "%s:%d:%s", identity.AccountType, identity.ID, identity.Password)
	return hex.EncodeToString(mac.Sum(nil))
}

func ValidateAccountToken(raw string) (*CustomClaims, *AccountIdentity, error) {
	claims, err := ParseToken(raw)
	if err != nil {
		return nil, nil, errors.New("登录凭证无效或已过期，请重新登录")
	}
	if claims.AuthVersion != 1 {
		return nil, nil, errors.New("登录身份规则已更新，请重新登录")
	}
	if claims.TestLogin && !TestLoginEnabled() {
		return nil, nil, errors.New("测试快捷登录已关闭，请使用微信授权登录")
	}
	identity, err := LoadAccount(claims.AccountType, claims.UserID)
	if err != nil {
		return nil, nil, fmt.Errorf("账号身份校验失败: %w", err)
	}
	if claims.Role != identity.Role || claims.RoleCode != identity.RoleCode || claims.TeacherID != identity.TeacherID ||
		!hmac.Equal([]byte(claims.CredentialVersion), []byte(accountCredentialVersion(identity))) {
		return nil, nil, errors.New("账号权限或密码已变更，请重新登录")
	}
	return claims, identity, nil
}
