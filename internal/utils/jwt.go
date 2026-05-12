package utils

import (
	"time"
	"ydxt-go/internal/config"

	"github.com/golang-jwt/jwt/v5"
)

type CustomClaims struct {
	UserID      uint64 `json:"user_id"`
	Role        int8   `json:"role"`
	AccountType string `json:"account_type"`
	RoleCode    string `json:"role_code"`
	TeacherID   uint64 `json:"teacher_id"`
	jwt.RegisteredClaims
}

// GenerateToken 生成 JWT Token
func GenerateToken(userID uint64, role int8, accountType, roleCode string, teacherID uint64) (string, error) {
	claims := CustomClaims{
		UserID:      userID,
		Role:        role,
		AccountType: accountType,
		RoleCode:    roleCode,
		TeacherID:   teacherID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(config.GlobalConfig.JWT.ExpireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "yuedi",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.GlobalConfig.JWT.Secret))
}

// ParseToken 解析 JWT Token
func ParseToken(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.GlobalConfig.JWT.Secret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, jwt.ErrSignatureInvalid
}
