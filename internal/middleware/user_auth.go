package middleware

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// UserAuth validates user JWT tokens and injects the userId into the context.
func UserAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := strings.TrimSpace(c.GetHeader("Authorization"))
		if raw == "" {
			log.Println("[AUTH] [ERROR] missing token")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}

		parts := strings.Split(raw, " ")
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			log.Println("[AUTH] [ERROR] invalid token format")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		token, err := jwt.Parse(parts[1], func(t *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			log.Println("[AUTH] [ERROR] token validation failed:", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			log.Println("[AUTH] [ERROR] token claims invalid")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		userIDValue, ok := claims["userId"].(string)
		if !ok || strings.TrimSpace(userIDValue) == "" {
			log.Println("[AUTH] [ERROR] userId claim missing")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		userID, err := primitive.ObjectIDFromHex(userIDValue)
		if err != nil {
			log.Println("[AUTH] [ERROR] invalid userId claim")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		log.Println("[AUTH] [INFO] user token validated")
		c.Set("userId", userID)
		c.Next()
	}
}
