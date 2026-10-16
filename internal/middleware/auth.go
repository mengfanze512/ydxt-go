package middleware

import (
	"net/http"
	"strings"

	"ydxt-go/internal/utils"

	"github.com/gin-gonic/gin"
)

// JWTAuth JWT 鉴权中间件
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 优先从 Authorization header 读取，其次从 X-Token 读取
		// 微信云托管的 HTTP 网关可能会过滤 Authorization 字段，因此支持 X-Token 兜底
		tokenStr := ""
		authHeader := c.Request.Header.Get("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				tokenStr = parts[1]
			}
		}
		if tokenStr == "" {
			tokenStr = c.Request.Header.Get("X-Token")
		}

		if tokenStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "请求头中 auth 为空"})
			c.Abort()
			return
		}

		claims, identity, err := utils.ValidateAccountToken(tokenStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": err.Error()})
			c.Abort()
			return
		}

		// 将当前请求的 userID 和 role 信息保存到请求的上下文 c 上
		c.Set("userID", claims.UserID)
		c.Set("role", claims.Role)
		c.Set("accountType", claims.AccountType)
		c.Set("roleCode", claims.RoleCode)
		c.Set("teacherID", claims.TeacherID)
		c.Set("verifiedClaims", claims)
		c.Set("accountIdentity", identity)

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

		role, ok := currentRole.(int8)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 403, "msg": "角色信息无效"})
			return
		}
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
