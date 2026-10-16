package utils

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"os"
	"time"
	"ydxt-go/internal/config"

	"github.com/golang-jwt/jwt/v5"
)

// Test access requires both an explicit server switch and a nontrivial shared key.
func TestLoginEnabled() bool {
	return os.Getenv("ENABLE_TEST_LOGIN") == "true" && len(os.Getenv("TEST_LOGIN_KEY")) >= 32
}

func CheckTestLoginKey(key string) bool {
	if !TestLoginEnabled() {
		return false
	}
	expected, actual := sha256.Sum256([]byte(os.Getenv("TEST_LOGIN_KEY"))), sha256.Sum256([]byte(key))
	return subtle.ConstantTimeCompare(expected[:], actual[:]) == 1
}

func GenerateTestToken(identity *AccountIdentity) (string, error) {
	if !TestLoginEnabled() {
		return "", errors.New("测试快捷登录未开启")
	}
	raw, err := GenerateToken(identity.ID, identity.Role, identity.AccountType, identity.RoleCode, identity.TeacherID)
	if err != nil {
		return "", err
	}
	claims, err := ParseToken(raw)
	if err != nil {
		return "", err
	}
	claims.TestLogin = true
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(time.Hour))
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(config.GlobalConfig.JWT.Secret))
}
