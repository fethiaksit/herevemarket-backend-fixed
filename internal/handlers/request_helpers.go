package handlers

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

func handlePanic(c *gin.Context, route string) {
	if r := recover(); r != nil {
		log.Printf("[%s] panic recovered: %v", route, r)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}

func ensureDBConnection(ctx context.Context, db *mongo.Database) error {
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	return db.Client().Ping(checkCtx, readpref.Primary())
}

func respondWithError(c *gin.Context, status int, route string, message string) {
	log.Printf("[%s] returning error %d: %s", route, status, message)
	c.AbortWithStatusJSON(status, gin.H{"error": message})
}
