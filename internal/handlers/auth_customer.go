package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"

	"backend/internal/models"
)

type RegisterRequest struct {
	FirstName string `json:"firstName" binding:"required"`
	LastName  string `json:"lastName" binding:"required"`
	Email     string `json:"email" binding:"required"`
	Password  string `json:"password" binding:"required"`
	Phone     string `json:"phone"`
}

type RegisterUserRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
	Name     string `json:"name" binding:"required"`
}

type LoginResponseUser struct {
	ID        string `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

type AuthTokens struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int64  `json:"expiresIn"`
}

func Register(db *mongo.Database, jwtSecret string, accessTTL time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Println("[AUTH] [ERROR] register read body failed:", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
			return
		}
		log.Printf("[AUTH] [DEBUG] register content-type: %s", c.GetHeader("Content-Type"))
		log.Printf("[AUTH] [DEBUG] register raw body: %s", string(body))

		if len(bytes.TrimSpace(body)) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "request body is required"})
			return
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			log.Println("[AUTH] [ERROR] register parse failed:", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload", "details": err.Error()})
			return
		}

		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		hasCustomerFields := false
		if _, ok := payload["firstName"]; ok {
			hasCustomerFields = true
		}
		if _, ok := payload["lastName"]; ok {
			hasCustomerFields = true
		}
		if _, ok := payload["phone"]; ok {
			hasCustomerFields = true
		}

		if hasCustomerFields {
			var customerReq RegisterRequest
			if err := c.ShouldBindBodyWith(&customerReq, binding.JSON); err != nil {
				respondValidationError(c, err)
				return
			}
			registerCustomer(c, db, customerReq)
			return
		}

		var userReq RegisterUserRequest
		if err := c.ShouldBindBodyWith(&userReq, binding.JSON); err != nil {
			respondValidationError(c, err)
			return
		}
		registerUser(c, db, userReq, jwtSecret, accessTTL)
	}
}

func respondValidationError(c *gin.Context, err error) {
	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		details := make([]string, 0, len(validationErrors))
		for _, fieldError := range validationErrors {
			field := lowerCamel(fieldError.Field())
			switch fieldError.Tag() {
			case "required":
				details = append(details, fmt.Sprintf("%s is required", field))
			default:
				details = append(details, fmt.Sprintf("%s is invalid", field))
			}
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation failed",
			"details": details,
		})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body", "details": err.Error()})
}

func lowerCamel(field string) string {
	if field == "" {
		return field
	}
	return strings.ToLower(field[:1]) + field[1:]
}

func Login(db *mongo.Database, jwtSecret string, accessTTL, refreshTTL time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
			return
		}

		email := strings.ToLower(strings.TrimSpace(req.Email))
		if email == "" || strings.TrimSpace(req.Password) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "email and password are required"})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var user models.User
		if err := db.Collection("users").FindOne(ctx, bson.M{"email": email}).Decode(&user); err == nil {
			if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
				log.Println("[AUTH] [ERROR] login invalid credentials for user")
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
				return
			}

			accessToken, err := issueUserToken(user.ID, user.Email, jwtSecret, accessTTL)
			if err != nil {
				log.Println("[AUTH] [ERROR] login token generation failed:", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
				return
			}

			log.Println("[AUTH] [INFO] user login succeeded:", user.Email)
			c.JSON(http.StatusOK, gin.H{
				"accessToken": accessToken,
				"user": gin.H{
					"id":    user.ID.Hex(),
					"name":  user.Name,
					"email": user.Email,
				},
			})
			return
		} else if err != mongo.ErrNoDocuments {
			log.Println("[AUTH] [ERROR] login user lookup failed:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		var customer models.Customer
		if err := db.Collection("customers").FindOne(ctx, bson.M{"email": email}).Decode(&customer); err != nil {
			log.Println("[AUTH] [ERROR] login invalid credentials for customer")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		if !customer.IsActive {
			log.Println("[AUTH] [ERROR] customer inactive:", email)
			c.JSON(http.StatusForbidden, gin.H{"error": "user is inactive"})
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(customer.PasswordHash), []byte(req.Password)); err != nil {
			log.Println("[AUTH] [ERROR] login invalid credentials for customer")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		tokens, err := issueTokens(c, db, customer.ID, customer.Email, customer.Role, jwtSecret, accessTTL, refreshTTL)
		if err != nil {
			log.Println("[AUTH] [ERROR] customer login token generation failed:", err)
			return
		}

		log.Println("[AUTH] [INFO] customer login succeeded:", customer.Email)
		c.JSON(http.StatusOK, gin.H{
			"accessToken":  tokens.AccessToken,
			"refreshToken": tokens.RefreshToken,
			"expiresIn":    tokens.ExpiresIn,
			"user": LoginResponseUser{
				ID:        customer.ID.Hex(),
				FirstName: customer.FirstName,
				LastName:  customer.LastName,
				Email:     customer.Email,
			},
		})
	}
}

func Refresh(db *mongo.Database, jwtSecret string, accessTTL, refreshTTL time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req RefreshRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
			return
		}

		plain := strings.TrimSpace(req.RefreshToken)
		if plain == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "refreshToken is required"})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		hash := hashToken(plain)
		var token models.RefreshToken
		if err := db.Collection("refresh_tokens").FindOne(ctx, bson.M{
			"tokenHash": hash,
			"revoked":   false,
		}).Decode(&token); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
			return
		}

		if time.Now().After(token.ExpiresAt) {
			_, _ = db.Collection("refresh_tokens").UpdateByID(ctx, token.ID, bson.M{"$set": bson.M{"revoked": true}})
			c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token expired"})
			return
		}

		var user models.Customer
		if err := db.Collection("customers").FindOne(ctx, bson.M{"_id": token.UserID}).Decode(&user); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
			return
		}

		if !user.IsActive {
			c.JSON(http.StatusForbidden, gin.H{"error": "user is inactive"})
			return
		}

		newTokens, err := issueTokens(c, db, user.ID, user.Email, user.Role, jwtSecret, accessTTL, refreshTTL)
		if err != nil {
			return
		}

		_, _ = db.Collection("refresh_tokens").UpdateByID(ctx, token.ID, bson.M{
			"$set": bson.M{
				"revoked":         true,
				"replacedByToken": newTokens.RefreshTokenID,
			},
		})

		c.JSON(http.StatusOK, gin.H{
			"accessToken":  newTokens.AccessToken,
			"refreshToken": newTokens.RefreshToken,
			"expiresIn":    newTokens.ExpiresIn,
			"user": LoginResponseUser{
				ID:        user.ID.Hex(),
				FirstName: user.FirstName,
				LastName:  user.LastName,
				Email:     user.Email,
			},
		})
	}
}

func Logout(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req RefreshRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
			return
		}

		plain := strings.TrimSpace(req.RefreshToken)
		if plain == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "refreshToken is required"})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		hash := hashToken(plain)
		res, err := db.Collection("refresh_tokens").UpdateOne(ctx, bson.M{
			"tokenHash": hash,
			"revoked":   false,
		}, bson.M{"$set": bson.M{"revoked": true}})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		if res.MatchedCount == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "logged out"})
	}
}

func registerCustomer(c *gin.Context, db *mongo.Database, req RegisterRequest) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || strings.TrimSpace(req.Password) == "" || strings.TrimSpace(req.FirstName) == "" || strings.TrimSpace(req.LastName) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "firstName, lastName, email and password are required"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	count, err := db.Collection("customers").CountDocuments(ctx, bson.M{"email": email})
	if err != nil {
		log.Println("[AUTH] [ERROR] customer register db error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	if count > 0 {
		log.Println("[AUTH] [ERROR] customer register email exists:", email)
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Println("[AUTH] [ERROR] customer register password hash failed:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "password hash failed"})
		return
	}

	now := time.Now()
	customer := models.Customer{
		FirstName:    strings.TrimSpace(req.FirstName),
		LastName:     strings.TrimSpace(req.LastName),
		Email:        email,
		Phone:        strings.TrimSpace(req.Phone),
		PasswordHash: string(hash),
		IsActive:     true,
		Role:         "user",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if _, err := db.Collection("customers").InsertOne(ctx, customer); err != nil {
		log.Println("[AUTH] [ERROR] customer register insert failed:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	log.Println("[AUTH] [INFO] customer registered:", email)
	c.JSON(http.StatusCreated, gin.H{"message": "User registered successfully"})
}

func registerUser(c *gin.Context, db *mongo.Database, req RegisterUserRequest, jwtSecret string, accessTTL time.Duration) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	name := strings.TrimSpace(req.Name)
	password := strings.TrimSpace(req.Password)
	if email == "" || name == "" || password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email, password and name are required"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	count, err := db.Collection("users").CountDocuments(ctx, bson.M{"email": email})
	if err != nil {
		log.Println("[AUTH] [ERROR] user register db error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	if count > 0 {
		log.Println("[AUTH] [ERROR] user register email exists:", email)
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Println("[AUTH] [ERROR] user register password hash failed:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "password hash failed"})
		return
	}

	now := time.Now()
	user := models.User{
		Email:        email,
		PasswordHash: string(hash),
		Name:         name,
		Addresses:    []models.Address{},
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	res, err := db.Collection("users").InsertOne(ctx, user)
	if err != nil {
		log.Println("[AUTH] [ERROR] user register insert failed:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	id, _ := res.InsertedID.(primitive.ObjectID)
	accessToken, err := issueUserToken(id, email, jwtSecret, accessTTL)
	if err != nil {
		log.Println("[AUTH] [ERROR] user register token generation failed:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
		return
	}

	log.Println("[AUTH] [INFO] user registered:", email)
	c.JSON(http.StatusCreated, gin.H{
		"accessToken": accessToken,
		"user": gin.H{
			"id":    id.Hex(),
			"name":  name,
			"email": email,
		},
	})
}

func issueUserToken(userID primitive.ObjectID, email, secret string, accessTTL time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"userId": userID.Hex(),
		"email":  email,
		"exp":    time.Now().Add(accessTTL).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

type issuedTokens struct {
	AccessToken    string
	RefreshToken   string
	RefreshTokenID primitive.ObjectID
	ExpiresIn      int64
}

func issueTokens(c *gin.Context, db *mongo.Database, userID primitive.ObjectID, email, role, secret string, accessTTL, refreshTTL time.Duration) (*issuedTokens, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":   userID.Hex(),
		"role":  role,
		"email": email,
		"exp":   now.Add(accessTTL).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err := token.SignedString([]byte(secret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
		return nil, err
	}

	plainRefresh := generateRefreshString()
	if plainRefresh == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
		return nil, errors.New("could not generate refresh token")
	}
	hashed := hashToken(plainRefresh)

	refresh := models.RefreshToken{
		UserID:    userID,
		TokenHash: hashed,
		ExpiresAt: now.Add(refreshTTL),
		Revoked:   false,
		CreatedAt: now,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := db.Collection("refresh_tokens").InsertOne(ctx, refresh)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return nil, err
	}

	refreshID := res.InsertedID.(primitive.ObjectID)
	return &issuedTokens{
		AccessToken:    accessToken,
		RefreshToken:   plainRefresh,
		RefreshTokenID: refreshID,
		ExpiresIn:      int64(accessTTL.Seconds()),
	}, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func generateRefreshString() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return hex.EncodeToString(buf)
}
