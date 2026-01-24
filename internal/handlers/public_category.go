package handlers

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"backend/internal/models"
)

func GetCategories(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		const route = "GET /categories"
		defer handlePanic(c, route)

		log.Printf("[%s] hit", route)

		if err := ensureDBConnection(c.Request.Context(), db); err != nil {
			respondWithError(c, http.StatusServiceUnavailable, route, "database unavailable")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		cursor, err := db.Collection("categories").Find(
			ctx,
			bson.M{"isActive": true},
		)
		if err != nil {
			respondWithError(c, http.StatusInternalServerError, route, "db error")
			return
		}
		defer cursor.Close(ctx)

		var categories []models.Category
		if err := cursor.All(ctx, &categories); err != nil {
			respondWithError(c, http.StatusInternalServerError, route, "decode error")
			return
		}

		log.Printf("[%s] returning %d categories", route, len(categories))
		c.JSON(http.StatusOK, categories)
	}
}
