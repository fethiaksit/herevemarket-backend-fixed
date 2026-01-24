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
- Pagination OPSİYONEL
- page + limit YOKSA → TÜM ÜRÜNLER
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
			filter["name"] = bson.M{"$regex": search, "$options": "i"}
		}

		findOptions := options.Find().
			SetSort(bson.D{{Key: "createdAt", Value: -1}})

		// 👉 Pagination SADECE page + limit varsa uygulanır
		pageStr := c.Query("page")
		limitStr := c.Query("limit")

		if pageStr != "" && limitStr != "" {
			page, limit, err := parsePaginationParams(pageStr, limitStr)
			if err != nil {
				respondWithError(c, http.StatusBadRequest, route, "invalid pagination params")
				return
			}

			findOptions.
				SetSkip((page - 1) * limit).
				SetLimit(limit)
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

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

		log.Printf("[%s] returning %d products", route, len(products))
		c.JSON(http.StatusOK, products)
	}
}

/*
GET /products/campaigns
- Pagination ZORUNLU
- response: data + pagination
*/
func GetCampaignProducts(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, limit, err := parsePaginationParams(c.Query("page"), c.Query("limit"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}

		cursor, err := db.Collection("products").Find(ctx, filter, findOptions)
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
				"page":  page,
				"limit": limit,
				"total": total,
			},
		})
	}
}
