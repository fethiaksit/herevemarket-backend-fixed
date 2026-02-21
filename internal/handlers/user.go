package handlers

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"backend/internal/models"
)

type addressRequest struct {
	Title     string `json:"title" binding:"required"`
	Detail    string `json:"detail" binding:"required"`
	Note      string `json:"note"`
	IsDefault bool   `json:"isDefault"`
}

type favoriteRequest struct {
	ProductID string `json:"productId" binding:"required"`
}

func GetMe(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := c.Get("userId")
		if !ok {
			log.Println("[AUTH] [ERROR] userId missing in context")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		var user models.User
		if err := db.Collection("users").FindOne(ctx, bson.M{"_id": userID}).Decode(&user); err != nil {
			log.Println("[AUTH] [ERROR] get me failed:", err)
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"id":        user.ID.Hex(),
			"email":     user.Email,
			"name":      user.Name,
			"phone":     user.Phone,
			"addresses": user.Addresses,
			"createdAt": user.CreatedAt,
			"updatedAt": user.UpdatedAt,
		})
	}
}

func GetUserAddresses(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := c.Get("userId")
		if !ok {
			log.Println("[ADDRESS] [ERROR] userId missing in context")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		var user models.User
		if err := db.Collection("users").FindOne(ctx, bson.M{"_id": userID}).Decode(&user); err != nil {
			log.Println("[ADDRESS] [ERROR] get addresses failed:", err)
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"addresses": user.Addresses})
	}
}

func CreateUserAddress(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDValue, ok := c.Get("userId")
		if !ok {
			log.Println("[ADDRESS] [ERROR] userId missing in context")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		userID := userIDValue.(primitive.ObjectID)

		var req addressRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Println("[ADDRESS] [ERROR] invalid address body:", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		var user models.User
		if err := db.Collection("users").FindOne(ctx, bson.M{"_id": userID}).Decode(&user); err != nil {
			log.Println("[ADDRESS] [ERROR] user not found:", err)
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}

		if req.IsDefault {
			for i := range user.Addresses {
				user.Addresses[i].IsDefault = false
			}
		}

		addressID, err := newAddressID()
		if err != nil {
			log.Println("[ADDRESS] [ERROR] address id generation failed:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "address id generation failed"})
			return
		}

		address := models.Address{
			ID:        addressID,
			Title:     strings.TrimSpace(req.Title),
			Detail:    strings.TrimSpace(req.Detail),
			Note:      strings.TrimSpace(req.Note),
			IsDefault: req.IsDefault,
		}

		user.Addresses = append(user.Addresses, address)
		user.UpdatedAt = time.Now()

		_, err = db.Collection("users").UpdateByID(ctx, userID, bson.M{
			"$set": bson.M{
				"addresses": user.Addresses,
				"updatedAt": user.UpdatedAt,
			},
		})
		if err != nil {
			log.Println("[ADDRESS] [ERROR] insert address failed:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		log.Println("[ADDRESS] [INFO] address created:", address.ID)
		c.JSON(http.StatusCreated, gin.H{"address": address})
	}
}

func newAddressID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}

	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		buf[0:4],
		buf[4:6],
		buf[6:8],
		buf[8:10],
		buf[10:16],
	), nil
}

func UpdateUserAddress(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDValue, ok := c.Get("userId")
		if !ok {
			log.Println("[ADDRESS] [ERROR] userId missing in context")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		userID := userIDValue.(primitive.ObjectID)

		var req addressRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Println("[ADDRESS] [ERROR] invalid address body:", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
			return
		}

		addressID := strings.TrimSpace(c.Param("id"))
		if addressID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid address id"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		var user models.User
		if err := db.Collection("users").FindOne(ctx, bson.M{"_id": userID}).Decode(&user); err != nil {
			log.Println("[ADDRESS] [ERROR] user not found:", err)
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}

		index := -1
		for i, addr := range user.Addresses {
			if addr.ID == addressID {
				index = i
				break
			}
		}
		if index == -1 {
			c.JSON(http.StatusNotFound, gin.H{"error": "address not found"})
			return
		}

		if req.IsDefault {
			for i := range user.Addresses {
				user.Addresses[i].IsDefault = false
			}
		}

		user.Addresses[index].Title = strings.TrimSpace(req.Title)
		user.Addresses[index].Detail = strings.TrimSpace(req.Detail)
		user.Addresses[index].Note = strings.TrimSpace(req.Note)
		user.Addresses[index].IsDefault = req.IsDefault
		user.UpdatedAt = time.Now()

		_, err := db.Collection("users").UpdateByID(ctx, userID, bson.M{
			"$set": bson.M{
				"addresses": user.Addresses,
				"updatedAt": user.UpdatedAt,
			},
		})
		if err != nil {
			log.Println("[ADDRESS] [ERROR] update address failed:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		log.Println("[ADDRESS] [INFO] address updated:", addressID)
		c.JSON(http.StatusOK, gin.H{"address": user.Addresses[index]})
	}
}

func DeleteUserAddress(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDValue, ok := c.Get("userId")
		if !ok {
			log.Println("[ADDRESS] [ERROR] userId missing in context")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		userID := userIDValue.(primitive.ObjectID)

		addressID := strings.TrimSpace(c.Param("id"))
		if addressID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid address id"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		var user models.User
		if err := db.Collection("users").FindOne(ctx, bson.M{"_id": userID}).Decode(&user); err != nil {
			log.Println("[ADDRESS] [ERROR] user not found:", err)
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}

		updated := make([]models.Address, 0, len(user.Addresses))
		found := false
		for _, addr := range user.Addresses {
			if addr.ID == addressID {
				found = true
				continue
			}
			updated = append(updated, addr)
		}
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "address not found"})
			return
		}

		user.UpdatedAt = time.Now()
		_, err := db.Collection("users").UpdateByID(ctx, userID, bson.M{
			"$set": bson.M{
				"addresses": updated,
				"updatedAt": user.UpdatedAt,
			},
		})
		if err != nil {
			log.Println("[ADDRESS] [ERROR] delete address failed:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		log.Println("[ADDRESS] [INFO] address deleted:", addressID)
		c.JSON(http.StatusOK, gin.H{"message": "address deleted"})
	}
}

func GetUserFavorites(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDValue, ok := c.Get("userId")
		if !ok {
			log.Println("[FAVORITE] [ERROR] userId missing in context")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		userID := userIDValue.(primitive.ObjectID)

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		var user models.User
		if err := db.Collection("users").FindOne(ctx, bson.M{"_id": userID}).Decode(&user); err != nil {
			log.Println("[FAVORITE] [ERROR] get favorites failed:", err)
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}

		if len(user.Favorites) == 0 {
			c.JSON(http.StatusOK, gin.H{"data": []models.Product{}})
			return
		}

		cursor, err := db.Collection("products").Find(ctx, bson.M{
			"_id":       bson.M{"$in": user.Favorites},
			"isDeleted": bson.M{"$ne": true},
		})
		if err != nil {
			log.Println("[FAVORITE] [ERROR] list favorites products failed:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		defer cursor.Close(ctx)

		products := make([]models.Product, 0, len(user.Favorites))
		if err := cursor.All(ctx, &products); err != nil {
			log.Println("[FAVORITE] [ERROR] decode favorites products failed:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		productByID := make(map[primitive.ObjectID]models.Product, len(products))
		for _, product := range products {
			productByID[product.ID] = product
		}

		ordered := make([]models.Product, 0, len(products))
		for _, favoriteID := range user.Favorites {
			if product, exists := productByID[favoriteID]; exists {
				ordered = append(ordered, product)
			}
		}

		c.JSON(http.StatusOK, gin.H{"data": ordered})
	}
}

func AddUserFavorite(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDValue, ok := c.Get("userId")
		if !ok {
			log.Println("[FAVORITE] [ERROR] userId missing in context")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		userID := userIDValue.(primitive.ObjectID)

		var req favoriteRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Println("[FAVORITE] [ERROR] invalid favorite body:", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
			return
		}

		productID, err := primitive.ObjectIDFromHex(strings.TrimSpace(req.ProductID))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid productId"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		if err := db.Collection("products").FindOne(ctx, bson.M{
			"_id":       productID,
			"isDeleted": bson.M{"$ne": true},
		}).Err(); err != nil {
			if err == mongo.ErrNoDocuments {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid productId"})
				return
			}
			log.Println("[FAVORITE] [ERROR] product lookup failed:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		_, err = db.Collection("users").UpdateByID(ctx, userID, bson.M{
			"$addToSet": bson.M{"favorites": productID},
			"$set":      bson.M{"updatedAt": time.Now()},
		})
		if err != nil {
			log.Println("[FAVORITE] [ERROR] add favorite failed:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "favorite updated"})
	}
}

func DeleteUserFavorite(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDValue, ok := c.Get("userId")
		if !ok {
			log.Println("[FAVORITE] [ERROR] userId missing in context")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		userID := userIDValue.(primitive.ObjectID)

		productID, err := primitive.ObjectIDFromHex(strings.TrimSpace(c.Param("productId")))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid productId"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		if err := db.Collection("products").FindOne(ctx, bson.M{
			"_id":       productID,
			"isDeleted": bson.M{"$ne": true},
		}).Err(); err != nil {
			if err == mongo.ErrNoDocuments {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid productId"})
				return
			}
			log.Println("[FAVORITE] [ERROR] product lookup failed:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		_, err = db.Collection("users").UpdateByID(ctx, userID, bson.M{
			"$pull": bson.M{"favorites": productID},
			"$set":  bson.M{"updatedAt": time.Now()},
		})
		if err != nil {
			log.Println("[FAVORITE] [ERROR] remove favorite failed:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "favorite updated"})
	}
}
