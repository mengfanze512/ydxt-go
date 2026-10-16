package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"ydxt-go/internal/config"
	"ydxt-go/internal/model"
	"ydxt-go/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func authRequest(router *gin.Engine, method, path, token string, body any) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Token", token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func authFixture(t *testing.T) *gin.Engine {
	db := deliveryTestDB(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("teacher-secret"), bcrypt.MinCost)
	mustCreate(t, db, &model.Teacher{ID: 10, Username: "teacher", Mobile: "13800000010", PasswordHash: string(hash), Status: 1})
	mustCreate(t, db, &model.Teacher{ID: 20, Username: "other", Mobile: "13800000020", PasswordHash: string(hash), Status: 1})
	hash, _ = bcrypt.GenerateFromPassword([]byte("admin-secret"), bcrypt.MinCost)
	mustCreate(t, db, &model.Admin{ID: 10, Username: "manager", Mobile: "13800000099", PasswordHash: string(hash), RoleCode: "super_admin", Status: 1})
	db.Model(&model.User{}).Where("id = 1").Updates(map[string]any{"phone": "13800000001", "password": "student-secret"})
	return InitRouter()
}

func loginToken(t *testing.T, router *gin.Engine, account, password string) string {
	t.Helper()
	rec := authRequest(router, "POST", "/api/v1/auth/phone-login", "", gin.H{"account": account, "password": password, "role": 9, "account_type": "admin"})
	var response struct {
		Data struct {
			Token string                `json:"token"`
			User  utils.AccountIdentity `json:"user"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &response)
	if rec.Code != 200 || response.Data.Token == "" {
		t.Fatalf("login failed: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "password") {
		t.Fatal("password exposed")
	}
	return response.Data.Token
}

func TestAccountLoginIdentityAndLegacyPassword(t *testing.T) {
	router := authFixture(t)
	for _, test := range []struct {
		Account, Password, Type string
		Role                    int8
	}{
		{"13800000001", "student-secret", "user", 1}, {"teacher", "teacher-secret", "teacher", 2}, {"manager", "admin-secret", "admin", 9},
	} {
		token := loginToken(t, router, test.Account, test.Password)
		claims, identity, err := utils.ValidateAccountToken(token)
		if err != nil || claims.Role != test.Role || identity.AccountType != test.Type {
			t.Fatalf("wrong account identity: %v", err)
		}
		rec := authRequest(router, "GET", "/api/v1/auth/me", token, nil)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"account_type":"`+test.Type+`"`) {
			t.Fatal(rec.Body.String())
		}
	}
	var user model.User
	model.DB.First(&user, 1)
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("student-secret")) != nil {
		t.Fatal("plaintext not upgraded")
	}
	for _, path := range []string{"/api/v1/admin/login", "/api/v1/auth/phone-login"} {
		rec := authRequest(router, "POST", path, "", gin.H{"username": "admin", "account": "teacher", "password": "123456"})
		if rec.Code == 200 {
			t.Fatal("default password bypass remains")
		}
	}
	rec := authRequest(router, "POST", "/api/v1/auth/phone-login", "", gin.H{"account": "teacher", "code": "123456"})
	if rec.Code != 400 {
		t.Fatal("fixed SMS accepted")
	}
	rec = authRequest(router, "POST", "/api/v1/admin/login", "", gin.H{"username": "teacher", "password": "teacher-secret"})
	if rec.Code != 200 {
		t.Fatal("teacher backend login failed: " + rec.Body.String())
	}
}

func TestLegacyPasswordUpgradeRejectsChangedCredential(t *testing.T) {
	authFixture(t)
	candidate := loginCandidate{AccountType: accountTypeUser, ID: 1, Password: "student-secret"}
	if err := model.DB.Model(&model.User{}).Where("id = ?", 1).Update("password", "replacement-secret").Error; err != nil {
		t.Fatal(err)
	}
	if err := upgradePassword(candidate, "student-secret"); err == nil {
		t.Fatal("stale password verification accepted")
	}
	var user model.User
	if err := model.DB.First(&user, 1).Error; err != nil {
		t.Fatal(err)
	}
	if user.Password != "replacement-secret" {
		t.Fatal("concurrent password change overwritten")
	}
}

func TestAccountChangesInvalidateCredentials(t *testing.T) {
	router := authFixture(t)
	teacherToken := loginToken(t, router, "teacher", "teacher-secret")
	for _, path := range []string{"/api/v1/admin/shop/goods", "/api/v1/admin/finance/settlements", "/api/v1/admin/teachers", "/api/v1/shop/orders"} {
		method := "GET"
		if path == "/api/v1/admin/teachers" {
			method = "POST"
		}
		if rec := authRequest(router, method, path, teacherToken, nil); rec.Code != 403 {
			t.Fatalf("teacher accessed %s: %s", path, rec.Body.String())
		}
	}
	rec := authRequest(router, "POST", "/api/v1/auth/change-password", teacherToken, gin.H{"old_password": "teacher-secret", "new_password": "updated-secret"})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if _, _, err := utils.ValidateAccountToken(teacherToken); err == nil {
		t.Fatal("old password token still valid")
	}
	newToken := loginToken(t, router, "teacher", "updated-secret")
	model.DB.Model(&model.Teacher{}).Where("id = 10").Update("status", 0)
	if _, _, err := utils.ValidateAccountToken(newToken); err == nil {
		t.Fatal("disabled teacher token still valid")
	}
	rec = authRequest(router, "POST", "/api/v1/auth/phone-login", "", gin.H{"account": "teacher", "password": "updated-secret"})
	if rec.Code != 403 {
		t.Fatal("disabled account login accepted")
	}
	adminToken := loginToken(t, router, "manager", "admin-secret")
	model.DB.Model(&model.Admin{}).Where("id = 10").Update("role_code", "admin")
	if _, _, err := utils.ValidateAccountToken(adminToken); err == nil {
		t.Fatal("old admin privilege remains")
	}
	claims, _ := utils.ParseToken(adminToken)
	claims.AuthVersion = 0
	legacy, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(config.GlobalConfig.JWT.Secret))
	if _, _, err := utils.ValidateAccountToken(legacy); err == nil {
		t.Fatal("legacy token accepted")
	}
}

type authMockTransport func(*http.Request) (*http.Response, error)

func (f authMockTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestWechatTeacherAndDuplicateIdentifiers(t *testing.T) {
	router := authFixture(t)
	old := wechatLoginClient
	t.Cleanup(func() { wechatLoginClient = old })
	config.GlobalConfig.Wechat.AppID, config.GlobalConfig.Wechat.AppSecret = "test", "test"
	wechatLoginClient = &http.Client{Transport: authMockTransport(func(req *http.Request) (*http.Response, error) {
		body := `{"openid":"test-openid","unionid":"test-unionid"}`
		if strings.Contains(req.URL.Path, "cgi-bin/token") {
			body = `{"access_token":"test-only"}`
		}
		if strings.Contains(req.URL.Path, "getuserphonenumber") {
			body = `{"phone_info":{"purePhoneNumber":"13800000010"}}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})}
	rec := authRequest(router, "POST", "/api/v1/auth/wx-login", "", gin.H{"code": "verified-code", "phone_code": "verified-phone"})
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"account_type":"teacher"`) {
		t.Fatal(rec.Body.String())
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("teacher-secret"), bcrypt.MinCost)
	model.DB.Model(&model.Admin{}).Where("id = 10").Updates(map[string]any{"username": "teacher", "mobile": "13800000010", "password_hash": string(hash)})
	rec = authRequest(router, "POST", "/api/v1/auth/wx-login", "", gin.H{"code": "verified-code", "phone_code": "verified-phone"})
	if rec.Code != 409 {
		t.Fatal("ambiguous WeChat identity accepted")
	}
	rec = authRequest(router, "POST", "/api/v1/auth/phone-login", "", gin.H{"account": "teacher", "password": "teacher-secret"})
	if rec.Code != 409 {
		t.Fatal("ambiguous password identity accepted")
	}
}
