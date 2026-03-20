package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const CtxUserIDKey = "userId"
const CtxUsernameKey = "username"

// Middleware 用于对 HTTP API 进行 JWT 鉴权，并将用户信息写入 Gin Context。
func Middleware(jwtSvc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearerToken(c.GetHeader("Authorization"))
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}

		claims, err := jwtSvc.ParseToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		c.Set(CtxUserIDKey, claims.UserID)
		c.Set(CtxUsernameKey, claims.Username)
		c.Next()
	}
}

// extractBearerToken 从 Authorization 头中提取 Bearer Token。
func extractBearerToken(authHeader string) string {
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		return ""
	}
	if !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
