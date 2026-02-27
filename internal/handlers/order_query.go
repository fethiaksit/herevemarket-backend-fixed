package handlers

import (
	"context"
	"errors"
	"fmt"
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

var validOrderStatuses = map[string]struct{}{
	"pending":   {},
	"approved":  {},
	"cancelled": {},
	"delivered": {},
}

type adminOrderResponse struct {
	ID            primitive.ObjectID   `json:"id" bson:"_id"`
	OrderCode     string               `json:"orderCode"`
	UserID        *primitive.ObjectID  `json:"userId,omitempty" bson:"userId"`
	UserPhone     string               `json:"userPhone,omitempty"`
	Items         []models.OrderItem   `json:"items" bson:"items"`
	TotalPrice    float64              `json:"totalPrice" bson:"totalPrice"`
	Customer      models.OrderCustomer `json:"customer" bson:"customer"`
	PaymentMethod string               `json:"paymentMethod" bson:"paymentMethod"`
	Status        string               `json:"status" bson:"status"`
	CreatedAt     time.Time            `json:"createdAt" bson:"createdAt"`
	UpdatedAt     *time.Time           `json:"updatedAt,omitempty" bson:"updatedAt,omitempty"`
}

type userPhoneRecord struct {
	ID    primitive.ObjectID `bson:"_id"`
	Phone string             `bson:"phone"`
}

func AdminGetOrders(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, limit, err := parsePaginationParams(c.Query("page"), c.Query("limit"))
		if err != nil {
			respondWithError(c, http.StatusBadRequest, "GET /admin/api/orders", err.Error())
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		filter, err := buildAdminOrdersFilter(ctx, db, c)
		if err != nil {
			respondWithError(c, http.StatusBadRequest, "GET /admin/api/orders", err.Error())
			return
		}

		total, err := db.Collection("orders").CountDocuments(ctx, filter)
		if err != nil {
			respondWithError(c, http.StatusInternalServerError, "GET /admin/api/orders", "db error")
			return
		}

		opts := options.Find().
			SetSort(bson.D{{Key: "createdAt", Value: -1}}).
			SetSkip((page - 1) * limit).
			SetLimit(limit)

		cursor, err := db.Collection("orders").Find(ctx, filter, opts)
		if err != nil {
			respondWithError(c, http.StatusInternalServerError, "GET /admin/api/orders", "db error")
			return
		}
		defer cursor.Close(ctx)

		var orders []adminOrderResponse
		if err := cursor.All(ctx, &orders); err != nil {
			respondWithError(c, http.StatusInternalServerError, "GET /admin/api/orders", "decode error")
			return
		}

		attachUserPhones(ctx, db, orders)
		enrichAdminOrders(orders)

		totalPages := int64(0)
		if total > 0 {
			totalPages = int64(math.Ceil(float64(total) / float64(limit)))
		}

		c.JSON(http.StatusOK, gin.H{
			"data": orders,
			"pagination": gin.H{
				"page":       page,
				"limit":      limit,
				"total":      total,
				"totalPages": totalPages,
			},
		})
	}
}

func AdminGetOrderByID(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		orderID, err := primitive.ObjectIDFromHex(c.Param("id"))
		if err != nil {
			respondWithError(c, http.StatusBadRequest, "GET /admin/api/orders/:id", "invalid id")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		var order adminOrderResponse
		if err := db.Collection("orders").FindOne(ctx, bson.M{"_id": orderID}).Decode(&order); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				respondWithError(c, http.StatusNotFound, "GET /admin/api/orders/:id", "order not found")
				return
			}
			respondWithError(c, http.StatusInternalServerError, "GET /admin/api/orders/:id", "db error")
			return
		}

		orders := []adminOrderResponse{order}
		attachUserPhones(ctx, db, orders)
		enrichAdminOrders(orders)
		c.JSON(http.StatusOK, orders[0])
	}
}

func AdminUpdateOrderStatus(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		orderID, err := primitive.ObjectIDFromHex(c.Param("id"))
		if err != nil {
			respondWithError(c, http.StatusBadRequest, "PUT /admin/api/orders/:id/status", "invalid id")
			return
		}

		var payload struct {
			Status string `json:"status"`
		}
		if err := c.ShouldBindJSON(&payload); err != nil {
			respondWithError(c, http.StatusBadRequest, "PUT /admin/api/orders/:id/status", "invalid payload")
			return
		}

		status := strings.ToLower(strings.TrimSpace(payload.Status))
		if _, ok := validOrderStatuses[status]; !ok {
			respondWithError(c, http.StatusBadRequest, "PUT /admin/api/orders/:id/status", "invalid status")
			return
		}

		now := time.Now()
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		update := bson.M{"$set": bson.M{"status": status, "updatedAt": now}}
		res, err := db.Collection("orders").UpdateOne(ctx, bson.M{"_id": orderID}, update)
		if err != nil {
			respondWithError(c, http.StatusInternalServerError, "PUT /admin/api/orders/:id/status", "db error")
			return
		}
		if res.MatchedCount == 0 {
			respondWithError(c, http.StatusNotFound, "PUT /admin/api/orders/:id/status", "order not found")
			return
		}

		var order adminOrderResponse
		if err := db.Collection("orders").FindOne(ctx, bson.M{"_id": orderID}).Decode(&order); err != nil {
			respondWithError(c, http.StatusInternalServerError, "PUT /admin/api/orders/:id/status", "db error")
			return
		}
		orders := []adminOrderResponse{order}
		attachUserPhones(ctx, db, orders)
		enrichAdminOrders(orders)
		c.JSON(http.StatusOK, gin.H{"message": "status updated", "data": orders[0]})
	}
}

func buildAdminOrdersFilter(ctx context.Context, db *mongo.Database, c *gin.Context) (bson.M, error) {
	filter := bson.M{}

	status := strings.ToLower(strings.TrimSpace(c.Query("status")))
	if status != "" {
		if _, ok := validOrderStatuses[status]; !ok {
			return nil, errors.New("invalid status filter")
		}
		filter["status"] = status
	}

	paymentMethod := strings.ToLower(strings.TrimSpace(c.Query("paymentMethod")))
	if paymentMethod != "" {
		if paymentMethod != "cash" && paymentMethod != "card" {
			return nil, errors.New("invalid paymentMethod filter")
		}
		filter["paymentMethod"] = paymentMethod
	}

	search := strings.TrimSpace(c.Query("search"))
	if search != "" {
		orderIDExpr := bson.M{"$expr": bson.M{"$regexMatch": bson.M{
			"input":   bson.M{"$toString": "$_id"},
			"regex":   fmt.Sprintf("%s$", strings.ToLower(search)),
			"options": "i",
		}}}

		userIDs, err := findUserIDsByPhone(ctx, db, search)
		if err != nil {
			return nil, errors.New("search failed")
		}

		orFilters := bson.A{orderIDExpr}
		if len(userIDs) > 0 {
			orFilters = append(orFilters, bson.M{"userId": bson.M{"$in": userIDs}})
		}
		filter["$or"] = orFilters
	}

	return filter, nil
}

func findUserIDsByPhone(ctx context.Context, db *mongo.Database, search string) ([]primitive.ObjectID, error) {
	cursor, err := db.Collection("users").Find(ctx, bson.M{
		"phone": bson.M{"$regex": primitive.Regex{Pattern: search, Options: "i"}},
	}, options.Find().SetProjection(bson.M{"_id": 1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var users []struct {
		ID primitive.ObjectID `bson:"_id"`
	}
	if err := cursor.All(ctx, &users); err != nil {
		return nil, err
	}

	ids := make([]primitive.ObjectID, 0, len(users))
	for _, user := range users {
		ids = append(ids, user.ID)
	}
	return ids, nil
}

func buildOrderCode(id primitive.ObjectID) string {
	hexID := id.Hex()
	if len(hexID) <= 8 {
		return strings.ToUpper(hexID)
	}
	return strings.ToUpper(hexID[len(hexID)-8:])
}

func enrichAdminOrders(orders []adminOrderResponse) {
	for i := range orders {
		orders[i].OrderCode = buildOrderCode(orders[i].ID)
	}
}

func attachUserPhones(ctx context.Context, db *mongo.Database, orders []adminOrderResponse) {
	if len(orders) == 0 {
		return
	}

	userIDSet := map[primitive.ObjectID]struct{}{}
	userIDs := make([]primitive.ObjectID, 0, len(orders))
	for _, order := range orders {
		if order.UserID == nil {
			continue
		}
		if _, exists := userIDSet[*order.UserID]; exists {
			continue
		}
		userIDSet[*order.UserID] = struct{}{}
		userIDs = append(userIDs, *order.UserID)
	}

	if len(userIDs) == 0 {
		return
	}

	userCursor, err := db.Collection("users").Find(ctx, bson.M{"_id": bson.M{"$in": userIDs}}, options.Find().SetProjection(bson.M{"phone": 1}))
	if err != nil {
		return
	}
	defer userCursor.Close(ctx)

	var users []userPhoneRecord
	if err := userCursor.All(ctx, &users); err != nil {
		return
	}

	phoneByUserID := make(map[primitive.ObjectID]string, len(users))
	for _, user := range users {
		phoneByUserID[user.ID] = user.Phone
	}

	for i := range orders {
		if orders[i].UserID == nil {
			continue
		}
		orders[i].UserPhone = phoneByUserID[*orders[i].UserID]
	}
}

func GetMyOrders(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDValue, exists := c.Get("userId")
		if !exists {
			respondWithError(c, http.StatusUnauthorized, "GET /user/orders", "unauthorized")
			return
		}

		userID, ok := userIDValue.(primitive.ObjectID)
		if !ok {
			respondWithError(c, http.StatusUnauthorized, "GET /user/orders", "unauthorized")
			return
		}

		page, limit, err := parsePaginationParams(c.Query("page"), c.Query("limit"))
		if err != nil {
			respondWithError(c, http.StatusBadRequest, "GET /user/orders", err.Error())
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		filter := bson.M{"userId": userID}
		total, err := db.Collection("orders").CountDocuments(ctx, filter)
		if err != nil {
			respondWithError(c, http.StatusInternalServerError, "GET /user/orders", "db error")
			return
		}

		opts := options.Find().
			SetSort(bson.D{{Key: "createdAt", Value: -1}}).
			SetSkip((page - 1) * limit).
			SetLimit(limit)

		cursor, err := db.Collection("orders").Find(ctx, filter, opts)
		if err != nil {
			respondWithError(c, http.StatusInternalServerError, "GET /user/orders", "db error")
			return
		}
		defer cursor.Close(ctx)

		var orders []models.Order
		if err := cursor.All(ctx, &orders); err != nil {
			respondWithError(c, http.StatusInternalServerError, "GET /user/orders", "decode error")
			return
		}

		totalPages := int64(0)
		if total > 0 {
			totalPages = int64(math.Ceil(float64(total) / float64(limit)))
		}

		c.JSON(http.StatusOK, gin.H{
			"data": orders,
			"pagination": gin.H{
				"page":       page,
				"limit":      limit,
				"total":      total,
				"totalPages": totalPages,
			},
		})
	}
}
