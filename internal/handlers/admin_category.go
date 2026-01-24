package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"backend/internal/models"
)

type CategoryCreateRequest struct {
	Name     string `json:"name" binding:"required"`
	IsActive *bool  `json:"isActive"`
}

type CategoryUpdateRequest struct {
	Name     *string `json:"name"`
	IsActive *bool   `json:"isActive"`
}

/*
GET /admin/categories
- Tüm kategoriler
- Admin paneli için (aktif/pasif dahil)
*/
func GetAllCategories(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		filter := bson.M{}

		// ?isActive=true/false
		if v := strings.TrimSpace(c.Query("isActive")); v != "" {
			filter["isActive"] = v == "true"
		}

		opts := options.Find().
			SetSort(bson.D{{Key: "createdAt", Value: -1}})

		cursor, err := db.Collection("categories").
			Find(context.Background(), filter, opts)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		defer cursor.Close(context.Background())

		var categories []models.Category
		if err := cursor.All(context.Background(), &categories); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "decode error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": categories,
		})
	}
}

/*
POST /admin/categories
- Aynı isimli kategori eklenemez
*/
func CreateCategory(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CategoryCreateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
			return
		}

		name := strings.TrimSpace(req.Name)
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
			return
		}

		// duplicate check
		count, err := db.Collection("categories").CountDocuments(
			context.Background(),
			bson.M{"name": name},
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		if count > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "category already exists"})
			return
		}

		isActive := true
		if req.IsActive != nil {
			isActive = *req.IsActive
		}

		category := models.Category{
			Name:      name,
			IsActive:  isActive,
			CreatedAt: time.Now(),
		}

		result, err := db.Collection("categories").
			InsertOne(context.Background(), category)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		category.ID = result.InsertedID.(primitive.ObjectID)

		c.JSON(http.StatusCreated, category)
	}
}

/*
PUT /admin/categories/:id
*/
func UpdateCategory(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := primitive.ObjectIDFromHex(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		var req CategoryUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
			return
		}

		update := bson.M{}

		if req.Name != nil {
			name := strings.TrimSpace(*req.Name)
			if name == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "name cannot be empty"})
				return
			}
			update["name"] = name
		}

		if req.IsActive != nil {
			update["isActive"] = *req.IsActive
		}

		if len(update) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
			return
		}

		var updated models.Category
		err = db.Collection("categories").
			FindOneAndUpdate(
				context.Background(),
				bson.M{"_id": id},
				bson.M{"$set": update},
				options.FindOneAndUpdate().SetReturnDocument(options.After),
			).
			Decode(&updated)

		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		c.JSON(http.StatusOK, updated)
	}
}

/*
DELETE /admin/categories/:id
- Soft delete
*/
func DeleteCategory(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := primitive.ObjectIDFromHex(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		result, err := db.Collection("categories").UpdateOne(
			context.Background(),
			bson.M{"_id": id},
			bson.M{"$set": bson.M{"isActive": false}},
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		if result.MatchedCount == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
			return
		}

		c.Status(http.StatusNoContent)
	}
}
