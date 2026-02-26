package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"

	"backend/internal/models"
)

type AdminLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func AdminLogin(db *mongo.Database, jwtSecret string, accessTTL, refreshTTL time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req AdminLoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
			return
		}

		email := strings.ToLower(strings.TrimSpace(req.Email))
		if email == "" || strings.TrimSpace(req.Password) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "email and password are required"})
			return
		}

		// 🔴 ESKİ: admins collection
		// ✅ YENİ: customers + role=admin
		var admin models.Customer
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := db.Collection("customers").FindOne(
			ctx,
			bson.M{
				"email": email,
				"role":  "admin",
			},
		).Decode(&admin)

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		// 🔑 Şifre kontrolü (Customer’daki HASH ile)
		if err := bcrypt.CompareHashAndPassword(
			[]byte(admin.PasswordHash),
			[]byte(req.Password),
		); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		tokens, err := issueTokens(c, db, admin.ID, admin.Email, "admin", jwtSecret, accessTTL, refreshTTL)
		if err != nil {
			return
		}

		setRefreshCookie(c, tokens.RefreshToken, refreshTTL)

		c.JSON(http.StatusOK, gin.H{
			"token":        tokens.AccessToken,
			"accessToken":  tokens.AccessToken,
			"refreshToken": tokens.RefreshToken,
			"expiresIn":    tokens.ExpiresIn,
		})
	}
}
