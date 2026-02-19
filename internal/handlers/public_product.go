package handlers

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

/*
GET /products
- Varsayılan pagination: page=1, limit=20
- Geçiş için: page/limit hiç verilmezse eski array response korunur
*/
func GetProducts(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		const route = "GET /products"
		defer handlePanic(c, route)

		log.Printf(
			"[%s] hit page=%s limit=%s category=%s search=%s",
			route,
			c.Query("page"),
			c.Query("limit"),
			c.Query("category"),
			c.Query("search"),
		)

		if err := ensureDBConnection(c.Request.Context(), db); err != nil {
			respondWithError(c, http.StatusServiceUnavailable, route, "database unavailable")
			return
		}

		// BASE FILTER
		filter := bson.M{
			"isActive":  bson.M{"$ne": false},
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

		pageStr := strings.TrimSpace(c.Query("page"))
		limitStr := strings.TrimSpace(c.Query("limit"))

		page, limit, err := parsePaginationParams(pageStr, limitStr)
		if err != nil {
			respondWithError(c, http.StatusBadRequest, route, "invalid pagination params")
			return
		}

		findOptions := options.Find().
			SetSort(bson.D{{Key: "createdAt", Value: -1}}).
			SetSkip((page - 1) * limit).
			SetLimit(limit)

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		total, err := db.Collection("products").CountDocuments(ctx, filter)
		if err != nil {
			respondWithError(c, http.StatusInternalServerError, route, "db error")
			return
		}

		cursor, err := db.Collection("products").Find(ctx, filter, findOptions)
		if err != nil {
			respondWithError(c, http.StatusInternalServerError, route, "db error")
			return
		}
		defer cursor.Close(ctx)

		products, err := decodeProducts(ctx, cursor)
		if err != nil {
			respondWithError(c, http.StatusInternalServerError, route, "decode error")
			return
		}

		totalPages := int64(0)
		if limit > 0 {
			totalPages = (total + limit - 1) / limit
		}

		_, hasPage := c.GetQuery("page")
		_, hasLimit := c.GetQuery("limit")

		log.Printf("[%s] returning %d products", route, len(products))
		if !hasPage && !hasLimit {
			c.JSON(http.StatusOK, products)
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

/*
GET /products/campaigns
- Varsayılan pagination: page=1, limit=20
- response: data + pagination
*/
func GetCampaignProducts(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		const route = "GET /products/campaigns"
		defer handlePanic(c, route)

		if err := ensureDBConnection(c.Request.Context(), db); err != nil {
			respondWithError(c, http.StatusServiceUnavailable, route, "database unavailable")
			return
		}

		pageStr := strings.TrimSpace(c.Query("page"))
		limitStr := strings.TrimSpace(c.Query("limit"))

		page, limit, err := parsePaginationParams(pageStr, limitStr)
		if err != nil {
			respondWithError(c, http.StatusBadRequest, route, "invalid pagination params")
			return
		}

		filter := bson.M{
			"isActive":   true,
			"isCampaign": true,
			"isDeleted":  bson.M{"$ne": true},
		}

		findOptions := options.Find().
			SetSkip((page - 1) * limit).
			SetLimit(limit).
			SetSort(bson.D{{Key: "createdAt", Value: -1}})

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		total, err := db.Collection("products").CountDocuments(ctx, filter)
		if err != nil {
			respondWithError(c, http.StatusInternalServerError, route, "db error")
			return
		}

		cursor, err := db.Collection("products").Find(ctx, filter, findOptions)
		if err != nil {
			respondWithError(c, http.StatusInternalServerError, route, "db error")
			return
		}
		defer cursor.Close(ctx)

		products, err := decodeProducts(ctx, cursor)
		if err != nil {
			respondWithError(c, http.StatusInternalServerError, route, "decode error")
			return
		}

		totalPages := int64(0)
		if limit > 0 {
			totalPages = (total + limit - 1) / limit
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
