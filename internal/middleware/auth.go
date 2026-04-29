package middleware

import (
	"net/http"
	"strings"

	"yuedi_edu/internal/utils"

	"github.com/gin-gonic/gin"
)

// JWTAuth JWT 鉴权中间件
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.Request.Header.Get("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "请求头中 auth 为空"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "请求头中 auth 格式有误"})
			c.Abort()
			return
		}

		claims, err := utils.ParseToken(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "无效的 Token"})
			c.Abort()
			return
		}

		// 将当前请求的 userID 和 role 信息保存到请求的上下文 c 上
		c.Set("userID", claims.UserID)
		c.Set("role", claims.Role)

		c.Next()
	}
}

// RoleAuth 角色权限中间件 (仅允许讲师/管理员访问)
func RoleAuth(allowRoles ...int8) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentRole, exists := c.Get("role")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "无法获取角色信息"})
			c.Abort()
			return
		}

		role := currentRole.(int8)
		isAllow := false
		for _, allowRole := range allowRoles {
			if role == allowRole {
				isAllow = true
				break
			}
		}

		if !isAllow {
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "权限不足"})
			c.Abort()
			return
		}

		c.Next()
	}
}
