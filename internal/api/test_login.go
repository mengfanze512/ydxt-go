package api

import (
	"net/http"
	"ydxt-go/internal/model"
	"ydxt-go/internal/utils"

	"github.com/gin-gonic/gin"
)

type testLoginTarget struct {
	Table, Column, AccountType, RoleCode string
}

func TestAccountLogin(c *gin.Context) {
	if !utils.TestLoginEnabled() {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "测试快捷登录未开启，请配置 ENABLE_TEST_LOGIN=true 和至少32字符的 TEST_LOGIN_KEY"})
		return
	}
	var req struct {
		Phone   string `json:"phone" binding:"required"`
		TestKey string `json:"test_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "测试登录参数错误: " + err.Error()})
		return
	}
	if !utils.CheckTestLoginKey(req.TestKey) {
		c.JSON(403, gin.H{"code": 403, "msg": "测试登录密钥不匹配"})
		return
	}
	target, ok := map[string]testLoginTarget{
		"13000000001": {"admins", "mobile", accountTypeAdmin, "super_admin"},
		"13000000101": {"admins", "mobile", accountTypeAdmin, "admin"},
		"13000000201": {"teachers", "mobile", accountTypeTeacher, "teacher"},
		"18532152083": {"users", "phone", accountTypeUser, "student"},
	}[req.Phone]
	if !ok {
		c.JSON(403, gin.H{"code": 403, "msg": "该手机号不在测试快捷登录白名单中"})
		return
	}
	if !deliveryDB(c) {
		return
	}
	var accounts []struct{ ID uint64 }
	if err := model.DB.Table(target.Table).Select("id").Where(target.Column+" = ?", req.Phone).Find(&accounts).Error; err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "查询测试账号失败: " + err.Error()})
		return
	}
	if len(accounts) == 0 {
		c.JSON(404, gin.H{"code": 404, "msg": "测试账号不存在，请先在 " + target.Table + " 配置手机号 " + req.Phone})
		return
	}
	if len(accounts) != 1 {
		c.JSON(409, gin.H{"code": 409, "msg": "该测试手机号匹配多个账号，请先整理重复记录"})
		return
	}
	identity, err := utils.LoadAccount(target.AccountType, accounts[0].ID)
	if err != nil {
		c.JSON(403, gin.H{"code": 403, "msg": err.Error()})
		return
	}
	if identity.RoleCode != target.RoleCode {
		c.JSON(403, gin.H{"code": 403, "msg": "测试账号的实际角色与快捷入口不符，请检查账号配置"})
		return
	}
	token, err := utils.GenerateTestToken(identity)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "msg": "生成测试凭证失败: " + err.Error()})
		return
	}
	home := "/pages/index/index"
	if identity.RoleCode == "super_admin" {
		home = "/pages/operation/dashboard"
	}
	c.JSON(200, gin.H{"code": 0, "msg": "测试账号登录成功", "data": gin.H{"token": token, "user": identity, "home": home}})
}
