package api

import (
	"encoding/json"
	"testing"
	"ydxt-go/internal/model"
	"ydxt-go/internal/utils"

	"github.com/gin-gonic/gin"
)

func TestShortcutLoginGuardsAndRealIdentities(t *testing.T) {
	router := authFixture(t)
	key := "isolated-test-key-at-least-32-characters"
	t.Setenv("ENABLE_TEST_LOGIN", "")
	t.Setenv("TEST_LOGIN_KEY", key)
	payload := gin.H{"phone": "13000000001", "test_key": key}
	if rec := authRequest(router, "POST", "/api/v1/auth/test-login", "", payload); rec.Code != 403 {
		t.Fatal("test login enabled by default")
	}
	t.Setenv("ENABLE_TEST_LOGIN", "true")
	t.Setenv("TEST_LOGIN_KEY", "short")
	if rec := authRequest(router, "POST", "/api/v1/auth/test-login", "", payload); rec.Code != 403 {
		t.Fatal("weak test key accepted")
	}
	t.Setenv("TEST_LOGIN_KEY", key)
	for _, body := range []gin.H{
		{"phone": "13000000001", "test_key": "wrong"},
		{"phone": "13800000099", "test_key": key},
	} {
		if rec := authRequest(router, "POST", "/api/v1/auth/test-login", "", body); rec.Code != 403 {
			t.Fatal("invalid test access accepted")
		}
	}
	if rec := authRequest(router, "POST", "/api/v1/auth/test-login", "", payload); rec.Code != 404 {
		t.Fatal("missing test account auto-created or not reported")
	}
	for _, update := range []struct {
		Table, Column, Phone string
		ID                   uint64
	}{
		{"admins", "mobile", "13000000001", 10},
		{"admins", "mobile", "13000000101", 99},
		{"teachers", "mobile", "13000000201", 10},
		{"users", "phone", "18532152083", 1},
	} {
		if err := model.DB.Table(update.Table).Where("id = ?", update.ID).Update(update.Column, update.Phone).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := model.DB.Model(&model.User{}).Where("id = ?", 1).Update("password", "").Error; err != nil {
		t.Fatal(err)
	}
	var testToken string
	for _, account := range []struct {
		Phone, Type, Role string
	}{
		{"13000000001", "admin", "super_admin"},
		{"13000000101", "admin", "admin"},
		{"13000000201", "teacher", "teacher"},
		{"18532152083", "user", "student"},
	} {
		rec := authRequest(router, "POST", "/api/v1/auth/test-login", "", gin.H{
			"phone": account.Phone, "test_key": key, "role": 9, "account_type": "admin",
		})
		var body struct {
			Data struct {
				Token string `json:"token"`
			} `json:"data"`
		}
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &body) != nil {
			t.Fatal(rec.Body.String())
		}
		claims, identity, err := utils.ValidateAccountToken(body.Data.Token)
		if err != nil || !claims.TestLogin || identity.AccountType != account.Type || identity.RoleCode != account.Role {
			t.Fatalf("incorrect real test identity: %v", err)
		}
		testToken = body.Data.Token
	}
	var student model.User
	if err := model.DB.First(&student, 1).Error; err != nil || student.Password != "" {
		t.Fatal("test login must not create or reset a student password")
	}
	normalToken := loginToken(t, router, "manager", "admin-secret")
	t.Setenv("ENABLE_TEST_LOGIN", "false")
	if _, _, err := utils.ValidateAccountToken(testToken); err == nil {
		t.Fatal("test token survived disabled test access")
	}
	if _, _, err := utils.ValidateAccountToken(normalToken); err != nil {
		t.Fatal("normal login disabled with test login")
	}
	t.Setenv("ENABLE_TEST_LOGIN", "true")
	model.DB.Model(&model.Admin{}).Where("id = ?", 10).Update("role_code", "admin")
	if rec := authRequest(router, "POST", "/api/v1/auth/test-login", "", payload); rec.Code != 403 {
		t.Fatal("shortcut granted super-admin to normal admin")
	}
	model.DB.Model(&model.Teacher{}).Where("id = ?", 10).Update("status", 0)
	if rec := authRequest(router, "POST", "/api/v1/auth/test-login", "", gin.H{"phone": "13000000201", "test_key": key}); rec.Code != 403 {
		t.Fatal("disabled test teacher accepted")
	}
}
