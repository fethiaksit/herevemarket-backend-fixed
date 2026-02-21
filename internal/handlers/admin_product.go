package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
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
	SaleEnabled *bool     `json:"saleEnabled"`
	SalePrice   *float64  `json:"salePrice"`
	CategoryIDs *[]string `json:"category_id"`
	Description *string   `json:"description"`
	Barcode     *string   `json:"barcode"`
	Brand       *string   `json:"brand"`
	Stock       *int      `json:"stock"`
	InStock     *bool     `json:"inStock"`
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

func sanitizeLogValue(value string, max int) string {
	trimmed := strings.TrimSpace(value)
	if max <= 0 {
		max = 80
	}
	if len(trimmed) <= max {
		return trimmed
	}
	return trimmed[:max] + "..."
}

func mapKeys(input map[string]interface{}) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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
			filter["$or"] = []bson.M{
				{"name": bson.M{"$regex": search, "$options": "i"}},
				{"brand": bson.M{"$regex": search, "$options": "i"}},
				{"description": bson.M{"$regex": search, "$options": "i"}},
				{"barcode": bson.M{"$regex": search, "$options": "i"}},
			}
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

		saleEnabled := false
		if input.SaleEnabledSet {
			saleEnabled = input.SaleEnabled
		}
		salePrice := 0.0
		if input.SalePriceSet {
			salePrice = input.SalePrice
		}

		if err := validateSaleFields(input.Price, saleEnabled, salePrice, input.SalePriceSet); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
			SaleEnabled: saleEnabled,
			SalePrice:   salePrice,
			IsOnSale:    isProductOnSale(input.Price, saleEnabled, salePrice),
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
		log.Println("UpdateProduct content-type:", c.GetHeader("Content-Type"))

		removeImage := false
		if removeRaw := strings.TrimSpace(c.Query("removeImage")); removeRaw != "" {
			parsedRemove, err := strconv.ParseBool(removeRaw)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "removeImage must be boolean"})
				return
			}
			removeImage = parsedRemove
		}

		if strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
			input, err := parseMultipartProductRequest(c)
			if err != nil {
				log.Println("UpdateProduct multipart error:", err)
				respondMultipartError(c, err)
				return
			}

			log.Printf("UpdateProduct image received: %t", input.ImageSet)
			log.Printf(
				"UpdateProduct form values: brand=%q barcode=%q stock=%d description=%q",
				sanitizeLogValue(input.Brand, 80),
				sanitizeLogValue(input.Barcode, 80),
				input.Stock,
				sanitizeLogValue(input.Description, 120),
			)

			var existing models.Product
			err = db.Collection("products").FindOne(
				context.Background(),
				bson.M{
					"_id":       id,
					"isDeleted": bson.M{"$ne": true},
				},
			).Decode(&existing)
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

			existingImagePath := strings.TrimSpace(existing.ImagePath)

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
			var saleInput saleUpdateInput
			if input.PriceSet {
				saleInput.Price = &input.Price
			}
			if input.SaleEnabledSet {
				saleInput.SaleEnabled = &input.SaleEnabled
			}
			if input.SalePriceSet {
				saleInput.SalePrice = &input.SalePrice
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
				updateUnset["image"] = ""
			} else if removeImage {
				updateUnset["imagePath"] = ""
				updateUnset["image"] = ""
			}
			if input.StockSet {
				if input.Stock < 0 {
					c.JSON(http.StatusBadRequest, gin.H{"error": "stock must be zero or greater"})
					return
				}
				updateSet["stock"] = input.Stock
				updateSet["inStock"] = input.Stock > 0
			} else if input.InStockSet {
				updateSet["inStock"] = input.InStock
			}
			if input.IsActiveSet {
				updateSet["isActive"] = input.IsActive
			}
			if input.IsCampaignSet {
				updateSet["isCampaign"] = input.IsCampaign
			}

			log.Printf(
				"UpdateProduct update fields: set=%v unset=%v",
				mapKeys(updateSet),
				mapKeys(updateUnset),
			)

			saleUpdate, err := resolveSaleUpdate(existing.Price, existing.SaleEnabled, existing.SalePrice, saleInput)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if saleUpdate.SetSaleEnabled {
				updateSet["saleEnabled"] = saleUpdate.SaleEnabled
			}
			if saleUpdate.SetSalePrice {
				updateSet["salePrice"] = saleUpdate.SalePrice
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

			if input.ImageSet && existingImagePath != "" && existingImagePath != input.ImagePath {
				if err := safeDeleteUpload(existingImagePath); err != nil {
					log.Printf("UpdateProduct old image delete failed: %v", err)
				}
			} else if removeImage && existingImagePath != "" {
				if err := safeDeleteUpload(existingImagePath); err != nil {
					log.Printf("UpdateProduct removeImage delete failed: %v", err)
				}
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
			updated.IsOnSale = isProductOnSale(updated.Price, updated.SaleEnabled, updated.SalePrice)
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
		if value, ok := raw["sale_enabled"]; ok {
			if _, exists := raw["saleEnabled"]; !exists {
				raw["saleEnabled"] = value
			}
		}
		if value, ok := raw["sale_price"]; ok {
			if _, exists := raw["salePrice"]; !exists {
				raw["salePrice"] = value
			}
		}

		if val, ok := raw["isCampaign"]; ok {
			if _, ok := val.(bool); !ok {
				log.Println("UpdateProduct RETURN 400:", "isCampaign must be boolean")
				c.JSON(http.StatusBadRequest, gin.H{"error": "isCampaign must be boolean"})
				return
			}
		}
		if val, ok := raw["saleEnabled"]; ok {
			if _, ok := val.(bool); !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": "saleEnabled must be boolean"})
				return
			}
		}

		normalizedBody, err := json.Marshal(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
			return
		}

		var req ProductUpdateRequest
		if err := json.Unmarshal(normalizedBody, &req); err != nil {
			log.Println("UpdateProduct bind error:", err)
			log.Println("UpdateProduct RETURN 400:", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
			return
		}
		log.Printf("UpdateProduct parsed request: %+v", req)

		var existingImagePath string
		if removeImage {
			var existing models.Product
			err := db.Collection("products").FindOne(
				context.Background(),
				bson.M{
					"_id":       id,
					"isDeleted": bson.M{"$ne": true},
				},
			).Decode(&existing)
			if err == mongo.ErrNoDocuments {
				c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
				return
			}
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
				return
			}
			existingImagePath = strings.TrimSpace(existing.ImagePath)
		}

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
		var saleInput saleUpdateInput
		if req.Price != nil {
			saleInput.Price = req.Price
		}
		if req.SaleEnabled != nil {
			saleInput.SaleEnabled = req.SaleEnabled
		}
		if req.SalePrice != nil {
			saleInput.SalePrice = req.SalePrice
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
			updateSet["inStock"] = *req.Stock > 0
		} else if req.InStock != nil {
			updateSet["inStock"] = *req.InStock
		}
		if req.IsActive != nil {
			updateSet["isActive"] = *req.IsActive
		}
		if req.IsCampaign != nil {
			updateSet["isCampaign"] = *req.IsCampaign
		}
		if removeImage {
			updateUnset["imagePath"] = ""
			updateUnset["image"] = ""
		}

		log.Printf(
			"UpdateProduct update fields: set=%v unset=%v",
			mapKeys(updateSet),
			mapKeys(updateUnset),
		)

		if req.Price != nil || req.SaleEnabled != nil || req.SalePrice != nil {
			var existing models.Product
			err := db.Collection("products").FindOne(
				context.Background(),
				bson.M{"_id": id, "isDeleted": bson.M{"$ne": true}},
			).Decode(&existing)
			if err == mongo.ErrNoDocuments {
				c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
				return
			}
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
				return
			}

			saleUpdate, err := resolveSaleUpdate(existing.Price, existing.SaleEnabled, existing.SalePrice, saleInput)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if saleUpdate.SetSaleEnabled {
				updateSet["saleEnabled"] = saleUpdate.SaleEnabled
			}
			if saleUpdate.SetSalePrice {
				updateSet["salePrice"] = saleUpdate.SalePrice
			}
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

		if removeImage && existingImagePath != "" {
			if err := safeDeleteUpload(existingImagePath); err != nil {
				log.Printf("UpdateProduct removeImage delete failed: %v", err)
			}
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
		updated.IsOnSale = isProductOnSale(updated.Price, updated.SaleEnabled, updated.SalePrice)
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

		var existing models.Product
		err = db.Collection("products").FindOne(
			context.Background(),
			bson.M{
				"_id":       id,
				"isDeleted": bson.M{"$ne": true},
			},
		).Decode(&existing)
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

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

		if err := safeDeleteUpload(existing.ImagePath); err != nil {
			log.Printf("DeleteProduct image delete failed: %v", err)
		}

		c.JSON(http.StatusOK, gin.H{"message": "product deleted"})
	}
}
