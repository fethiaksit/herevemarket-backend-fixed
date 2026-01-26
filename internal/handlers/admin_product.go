package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
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

/* =======================
   REQUEST MODELLERİ
======================= */

type ProductUpdateRequest struct {
	Name        *string   `json:"name"`
	Price       *float64  `json:"price"`
	CategoryIDs *[]string `json:"category_id"`
	Description *string   `json:"description"`
	Barcode     *string   `json:"barcode"`
	Brand       *string   `json:"brand"`
	Stock       *int      `json:"stock"`
	IsActive    *bool     `json:"isActive"`
	IsCampaign  *bool     `json:"isCampaign"`
}

/* =======================
   HELPERS
======================= */

func normalizeCategories(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)

	for _, v := range values {
		name := strings.TrimSpace(v)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

func resolveCategoryNamesByIDs(ctx context.Context, db *mongo.Database, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("category_id required")
	}

	seen := map[primitive.ObjectID]struct{}{}
	ordered := make([]primitive.ObjectID, 0, len(ids))
	unique := make([]primitive.ObjectID, 0, len(ids))

	for _, raw := range ids {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		objectID, err := primitive.ObjectIDFromHex(value)
		if err != nil {
			return nil, fmt.Errorf("invalid category_id: %s", value)
		}
		if _, ok := seen[objectID]; ok {
			continue
		}
		seen[objectID] = struct{}{}
		ordered = append(ordered, objectID)
		unique = append(unique, objectID)
	}

	if len(unique) == 0 {
		return nil, fmt.Errorf("category_id required")
	}

	cursor, err := db.Collection("categories").Find(ctx, bson.M{"_id": bson.M{"$in": unique}})
	if err != nil {
		return nil, err
	}

	var categories []models.Category
	if err := cursor.All(ctx, &categories); err != nil {
		return nil, err
	}

	nameByID := make(map[primitive.ObjectID]string, len(categories))
	for _, category := range categories {
		nameByID[category.ID] = category.Name
	}

	names := make([]string, 0, len(ordered))
	for _, objectID := range ordered {
		name, ok := nameByID[objectID]
		if !ok {
			return nil, fmt.Errorf("category not found: %s", objectID.Hex())
		}
		names = append(names, name)
	}

	return names, nil
}

/* =======================
   GET (ADMIN) – LIST
======================= */

func GetAllProducts(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, limit, err := parsePaginationParams(
			c.Query("page"),
			c.Query("limit"),
		)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		filter := bson.M{
			"isDeleted": bson.M{"$ne": true},
		}

		if category := strings.TrimSpace(c.Query("category")); category != "" {
			filter["category"] = bson.M{"$in": []string{category}}
		}

		if search := strings.TrimSpace(c.Query("search")); search != "" {
			filter["name"] = bson.M{"$regex": search, "$options": "i"}
		}

		if isActive := strings.TrimSpace(c.Query("isActive")); isActive != "" {
			filter["isActive"] = strings.EqualFold(isActive, "true")
		}

		ctx := context.Background()

		total, err := db.Collection("products").CountDocuments(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		totalPages := int64(0)
		if total > 0 {
			totalPages = int64(math.Ceil(float64(total) / float64(limit)))
		}

		opts := options.Find().
			SetSkip((page - 1) * limit).
			SetLimit(limit).
			SetSort(bson.D{{Key: "createdAt", Value: -1}})

		cursor, err := db.Collection("products").Find(ctx, filter, opts)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		defer cursor.Close(ctx)

		products, err := decodeProducts(ctx, cursor)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "decode error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": products,
			"pagination": gin.H{
				"page":       page,
				"limit":      limit,
				"total":      total,
				"totalPages": totalPages,
			},
		})
	}
}

/* =======================
   CREATE
======================= */

func CreateProduct(db *mongo.Database) gin.HandlerFunc {

	return func(c *gin.Context) {
		log.Println("CreateProduct: request received")
		log.Println("=== CREATE PRODUCT HIT ===")
		log.Println("Content-Type:", c.GetHeader("Content-Type"))
		log.Println("Form:", c.Request.MultipartForm)
		if !strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
			c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": "multipart/form-data required"})
			return
		}

		input, err := parseMultipartProductRequest(c)
		if err != nil {
			log.Println("CreateProduct multipart error:", err)
			respondMultipartError(c, err)
			return
		}

		name := strings.TrimSpace(input.Name)
		if !input.NameSet || name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
			return
		}

		if !input.PriceSet || input.Price <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid price"})
			return
		}

		if !input.CategoryIDSet {
			c.JSON(http.StatusBadRequest, gin.H{"error": "category_id required"})
			return
		}

		categoryNames, err := resolveCategoryNamesByIDs(context.Background(), db, input.CategoryIDs)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		categories := normalizeCategories(categoryNames)

		if !input.StockSet {
			c.JSON(http.StatusBadRequest, gin.H{"error": "stock required"})
			return
		}

		if input.Stock < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "stock must be zero or greater"})
			return
		}

		if !input.ImageSet || strings.TrimSpace(input.ImagePath) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "image required"})
			return
		}

		isActive := true
		if input.IsActiveSet {
			isActive = input.IsActive
		}

		isCampaign := false
		if input.IsCampaignSet {
			isCampaign = input.IsCampaign
		}

		now := time.Now()
		barcode := strings.TrimSpace(input.Barcode)
		brand := strings.TrimSpace(input.Brand)
		description := strings.TrimSpace(input.Description)

		product := models.Product{
			Name:        name,
			Price:       input.Price,
			Category:    models.StringList(categories),
			Description: description,
			Barcode:     barcode,
			Brand:       brand,
			ImagePath:   input.ImagePath,
			Stock:       input.Stock,
			InStock:     input.Stock > 0,
			IsActive:    isActive,
			IsCampaign:  isCampaign,
			IsDeleted:   false,
			CreatedAt:   now,
		}

		log.Printf("CreateProduct inserting product: %+v", product)
		res, err := db.Collection("products").InsertOne(context.Background(), product)
		if err != nil {
			log.Println("CreateProduct insert error:", err)
			log.Println("CreateProduct RETURN 500:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		product.ID = res.InsertedID.(primitive.ObjectID)
		log.Println("CreateProduct insert success:", res.InsertedID)
		c.JSON(http.StatusCreated, product)
	}
}

/* =======================
   UPDATE
======================= */

func UpdateProduct(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := primitive.ObjectIDFromHex(c.Param("id"))
		if err != nil {
			log.Println("UpdateProduct RETURN 400:", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		log.Println("UpdateProduct request received for id:", id.Hex())

		if strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
			input, err := parseMultipartProductRequest(c)
			if err != nil {
				log.Println("UpdateProduct multipart error:", err)
				respondMultipartError(c, err)
				return
			}

			updateSet := bson.M{}
			updateUnset := bson.M{}

			if input.NameSet {
				name := strings.TrimSpace(input.Name)
				if name == "" {
					c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
					return
				}
				updateSet["name"] = name
			}
			if input.PriceSet {
				if input.Price <= 0 {
					c.JSON(http.StatusBadRequest, gin.H{"error": "invalid price"})
					return
				}
				updateSet["price"] = input.Price
			}
			if input.CategoryIDSet {
				categoryNames, err := resolveCategoryNamesByIDs(context.Background(), db, input.CategoryIDs)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				updateSet["category"] = models.StringList(normalizeCategories(categoryNames))
			}
			if input.DescriptionSet {
				updateSet["description"] = strings.TrimSpace(input.Description)
			}
			if input.BarcodeSet {
				barcode := strings.TrimSpace(input.Barcode)
				if barcode == "" {
					updateUnset["barcode"] = ""
				} else {
					updateSet["barcode"] = barcode
				}
			}
			if input.BrandSet {
				updateSet["brand"] = strings.TrimSpace(input.Brand)
			}
			if input.ImageSet && strings.TrimSpace(input.ImagePath) != "" {
				updateSet["imagePath"] = input.ImagePath
			}
			if input.StockSet {
				if input.Stock < 0 {
					c.JSON(http.StatusBadRequest, gin.H{"error": "stock must be zero or greater"})
					return
				}
				updateSet["stock"] = input.Stock
			}
			if input.IsActiveSet {
				updateSet["isActive"] = input.IsActive
			}
			if input.IsCampaignSet {
				updateSet["isCampaign"] = input.IsCampaign
			}

			if len(updateSet) == 0 && len(updateUnset) == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
				return
			}

			update := bson.M{}
			if len(updateSet) > 0 {
				update["$set"] = updateSet
			}
			if len(updateUnset) > 0 {
				update["$unset"] = updateUnset
			}

			result, err := db.Collection("products").UpdateOne(
				context.Background(),
				bson.M{
					"_id":       id,
					"isDeleted": bson.M{"$ne": true},
				},
				update,
			)

			if err != nil {
				log.Println("UpdateProduct update error:", err)
				log.Println("UpdateProduct RETURN 500:", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
				return
			}

			log.Printf("UpdateProduct update result: matched=%d modified=%d", result.MatchedCount, result.ModifiedCount)

			if result.MatchedCount == 0 {
				log.Println("UpdateProduct RETURN 404:", "product not found")
				c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
				return
			}

			var updated models.Product
			err = db.Collection("products").FindOne(
				context.Background(),
				bson.M{
					"_id":       id,
					"isDeleted": bson.M{"$ne": true},
				},
			).Decode(&updated)

			if err == mongo.ErrNoDocuments {
				log.Println("UpdateProduct RETURN 404:", err)
				c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
				return
			}
			if err != nil {
				log.Println("UpdateProduct find error:", err)
				log.Println("UpdateProduct RETURN 500:", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
				return
			}

			updated.InStock = updated.Stock > 0
			c.JSON(http.StatusOK, updated)
			return
		}

		body, err := c.GetRawData()
		if err != nil {
			log.Println("UpdateProduct read body error:", err)
			log.Println("UpdateProduct RETURN 400:", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
			return
		}
		log.Println("UpdateProduct raw body:", string(body))

		var raw map[string]interface{}
		if err := json.Unmarshal(body, &raw); err != nil {
			log.Println("UpdateProduct raw json error:", err)
			log.Println("UpdateProduct RETURN 400:", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
			return
		}

		if val, ok := raw["isCampaign"]; ok {
			if _, ok := val.(bool); !ok {
				log.Println("UpdateProduct RETURN 400:", "isCampaign must be boolean")
				c.JSON(http.StatusBadRequest, gin.H{"error": "isCampaign must be boolean"})
				return
			}
		}

		var req ProductUpdateRequest
		if err := json.Unmarshal(body, &req); err != nil {
			log.Println("UpdateProduct bind error:", err)
			log.Println("UpdateProduct RETURN 400:", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
			return
		}
		log.Printf("UpdateProduct parsed request: %+v", req)

		updateSet := bson.M{}
		updateUnset := bson.M{}

		if req.Name != nil {
			updateSet["name"] = *req.Name
		}
		if req.Price != nil {
			if *req.Price <= 0 {
				log.Println("UpdateProduct RETURN 400:", "invalid price")
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid price"})
				return
			}
			updateSet["price"] = *req.Price
		}
		if req.CategoryIDs != nil {
			categoryNames, err := resolveCategoryNamesByIDs(context.Background(), db, *req.CategoryIDs)
			if err != nil {
				log.Println("UpdateProduct RETURN 400:", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			updateSet["category"] = models.StringList(normalizeCategories(categoryNames))
		}
		if req.Description != nil {
			updateSet["description"] = strings.TrimSpace(*req.Description)
		}
		if req.Barcode != nil {
			barcode := strings.TrimSpace(*req.Barcode)
			if barcode == "" {
				updateUnset["barcode"] = ""
			} else {
				updateSet["barcode"] = barcode
			}
		}
		if req.Brand != nil {
			updateSet["brand"] = strings.TrimSpace(*req.Brand)
		}
		if req.Stock != nil {
			if *req.Stock < 0 {
				log.Println("UpdateProduct RETURN 400:", "stock must be zero or greater")
				c.JSON(http.StatusBadRequest, gin.H{"error": "stock must be zero or greater"})
				return
			}
			updateSet["stock"] = *req.Stock
		}
		if req.IsActive != nil {
			updateSet["isActive"] = *req.IsActive
		}
		if req.IsCampaign != nil {
			updateSet["isCampaign"] = *req.IsCampaign
		}

		if len(updateSet) == 0 && len(updateUnset) == 0 {
			log.Println("UpdateProduct RETURN 400:", "no fields to update")
			c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
			return
		}

		update := bson.M{}
		if len(updateSet) > 0 {
			update["$set"] = updateSet
		}
		if len(updateUnset) > 0 {
			update["$unset"] = updateUnset
		}
		log.Printf("UpdateProduct update document: %+v", update)

		result, err := db.Collection("products").UpdateOne(
			context.Background(),
			bson.M{
				"_id":       id,
				"isDeleted": bson.M{"$ne": true},
			},
			update,
		)

		if err != nil {
			log.Println("UpdateProduct update error:", err)
			log.Println("UpdateProduct RETURN 500:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		log.Printf("UpdateProduct update result: matched=%d modified=%d", result.MatchedCount, result.ModifiedCount)

		if result.MatchedCount == 0 {
			log.Println("UpdateProduct RETURN 404:", "product not found")
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}

		var updated models.Product
		err = db.Collection("products").FindOne(
			context.Background(),
			bson.M{
				"_id":       id,
				"isDeleted": bson.M{"$ne": true},
			},
		).Decode(&updated)

		if err == mongo.ErrNoDocuments {
			log.Println("UpdateProduct RETURN 404:", err)
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}
		if err != nil {
			log.Println("UpdateProduct find error:", err)
			log.Println("UpdateProduct RETURN 500:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		updated.InStock = updated.Stock > 0
		c.JSON(http.StatusOK, updated)
	}
}

/* =======================
   DELETE (SOFT)
======================= */

func DeleteProduct(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := primitive.ObjectIDFromHex(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		now := time.Now()

		res, err := db.Collection("products").UpdateOne(
			context.Background(),
			bson.M{
				"_id":       id,
				"isDeleted": bson.M{"$ne": true},
			},
			bson.M{"$set": bson.M{
				"isDeleted": true,
				"deletedAt": now,
				"isActive":  false,
			}},
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		if res.MatchedCount == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "product deleted"})
	}
}
