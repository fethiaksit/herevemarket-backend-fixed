package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"backend/internal/models"
)

/* =========================
   REQUEST DTOs
========================= */

type createOrderItemRequest struct {
	ProductID string  `json:"productId" binding:"required"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity" binding:"required"`
}

type createOrderCustomerRequest struct {
	Title  string `json:"title" binding:"required"`
	Detail string `json:"detail" binding:"required"`
	Note   string `json:"note"`
}

type createOrderPaymentMethodRequest struct {
	ID    string `json:"id" binding:"required"`
	Label string `json:"label"`
}

type createOrderRequest struct {
	Items         []createOrderItemRequest        `json:"items" binding:"required"`
	TotalPrice    float64                         `json:"totalPrice"`
	Customer      createOrderCustomerRequest      `json:"customer" binding:"required"`
	PaymentMethod createOrderPaymentMethodRequest `json:"paymentMethod" binding:"required"`
}

/* =========================
   CREATE ORDER
========================= */

func CreateOrder(db *mongo.Database, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		const route = "POST /orders"
		defer handlePanic(c, route)

		if err := ensureDBConnection(c.Request.Context(), db); err != nil {
			respondWithError(c, http.StatusServiceUnavailable, route, "database unavailable")
			return
		}

		var req createOrderRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			respondWithError(c, http.StatusBadRequest, route, "invalid request body")
			return
		}

		userID, err := userIDFromHeader(c.GetHeader("Authorization"), jwtSecret)
		if err != nil {
			log.Println("[ORDER] [ERROR] token validation failed:", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		order, err := buildOrderFromRequest(req)
		if err != nil {
			respondWithError(c, http.StatusBadRequest, route, err.Error())
			return
		}
		order.UserID = userID

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		session, err := db.Client().StartSession()
		if err != nil {
			respondWithError(c, http.StatusInternalServerError, route, "db error")
			return
		}
		defer session.EndSession(ctx)

		var orderID primitive.ObjectID
		_, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
			calculatedItems := make([]models.OrderItem, 0, len(order.Items))
			calculatedTotal := 0.0

			for _, item := range order.Items {
				var product models.Product
				err := db.Collection("products").FindOne(
					sessCtx,
					bson.M{
						"_id":       item.ProductID,
						"isDeleted": bson.M{"$ne": true},
					},
				).Decode(&product)
				if err == mongo.ErrNoDocuments {
					return nil, productNotFoundError{ProductID: item.ProductID}
				}
				if err != nil {
					return nil, err
				}

				if product.Stock < item.Quantity {
					return nil, outOfStockError{
						ProductID: item.ProductID,
						Available: product.Stock,
						Requested: item.Quantity,
					}
				}

				unitPrice := effectiveProductPrice(product.Price, product.SaleEnabled, product.SalePrice)
				calculatedItems = append(calculatedItems, models.OrderItem{
					ProductID: item.ProductID,
					Name:      product.Name,
					Price:     unitPrice,
					Quantity:  item.Quantity,
				})
				calculatedTotal += unitPrice * float64(item.Quantity)

				filter := bson.M{
					"_id":       item.ProductID,
					"isDeleted": bson.M{"$ne": true},
					"stock":     bson.M{"$gte": item.Quantity},
				}
				update := bson.M{"$inc": bson.M{"stock": -item.Quantity}}

				res, err := db.Collection("products").UpdateOne(sessCtx, filter, update)
				if err != nil {
					return nil, err
				}
				if res.MatchedCount == 0 {
					return nil, outOfStockError{
						ProductID: item.ProductID,
						Available: product.Stock,
						Requested: item.Quantity,
					}
				}
			}

			order.Items = calculatedItems
			order.TotalPrice = calculatedTotal

			res, err := db.Collection("orders").InsertOne(sessCtx, order)
			if err != nil {
				return nil, err
			}
			if id, ok := res.InsertedID.(primitive.ObjectID); ok {
				orderID = id
			}
			return nil, nil
		})
		if err != nil {
			var stockErr outOfStockError
			if errors.As(err, &stockErr) {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":     "Stok yetersiz",
					"productId": stockErr.ProductID.Hex(),
					"available": stockErr.Available,
					"requested": stockErr.Requested,
				})
				return
			}
			var notFoundErr productNotFoundError
			if errors.As(err, &notFoundErr) {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":     "Ürün bulunamadı",
					"productId": notFoundErr.ProductID.Hex(),
				})
				return
			}
			respondWithError(c, http.StatusInternalServerError, route, "db error")
			return
		}

		if !orderID.IsZero() {
			order.ID = orderID
		}

		if userID != nil {
			log.Println("[ORDER] [INFO] order created for user:", userID.Hex())
		} else {
			log.Println("[ORDER] [INFO] guest order created")
		}

		c.JSON(http.StatusCreated, gin.H{
			"orderId": order.ID.Hex(),
			"message": "order created",
		})
	}
}

/* =========================
   GET ORDERS
========================= */

func GetOrders(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		cursor, err := db.Collection("orders").Find(ctx, bson.M{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Orders could not be fetched"})
			return
		}
		defer cursor.Close(ctx)

		var orders []models.Order
		if err := cursor.All(ctx, &orders); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse orders"})
			return
		}

		c.JSON(http.StatusOK, orders)
	}
}

/* =========================
   BUILD ORDER
========================= */

func buildOrderFromRequest(req createOrderRequest) (models.Order, error) {
	if len(req.Items) == 0 {
		return models.Order{}, errors.New("at least one item is required")
	}

	if req.PaymentMethod.ID != "cash" && req.PaymentMethod.ID != "card" {
		return models.Order{}, errors.New("invalid payment method")
	}

	items := make([]models.OrderItem, 0, len(req.Items))
	var total float64

	for _, item := range req.Items {
		productID, err := primitive.ObjectIDFromHex(item.ProductID)
		if err != nil {
			return models.Order{}, errors.New("invalid productId")
		}

		if item.Quantity <= 0 {
			return models.Order{}, errors.New("quantity must be greater than zero")
		}

		items = append(items, models.OrderItem{
			ProductID: productID,
			Name:      strings.TrimSpace(item.Name),
			Price:     0,
			Quantity:  item.Quantity,
		})
	}

	order := models.Order{
		Items:         items,
		TotalPrice:    total,
		Customer:      models.OrderCustomer(req.Customer),
		PaymentMethod: req.PaymentMethod.ID, // 🔥 sadece "card" / "cash" kaydedilir
		Status:        "pending",
		CreatedAt:     time.Now(),
	}

	return order, nil
}

func userIDFromHeader(header, secret string) (*primitive.ObjectID, error) {
	raw := strings.TrimSpace(header)
	if raw == "" {
		return nil, nil
	}

	parts := strings.Split(raw, " ")
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, errors.New("invalid token format")
	}

	token, err := jwt.Parse(parts[1], func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	userIDValue, ok := claims["userId"].(string)
	if !ok || strings.TrimSpace(userIDValue) == "" {
		return nil, errors.New("userId claim missing")
	}

	userID, err := primitive.ObjectIDFromHex(userIDValue)
	if err != nil {
		return nil, errors.New("invalid userId")
	}

	return &userID, nil
}

type outOfStockError struct {
	ProductID primitive.ObjectID
	Available int
	Requested int
}

func (e outOfStockError) Error() string {
	return "product out of stock"
}

type productNotFoundError struct {
	ProductID primitive.ObjectID
}

func (e productNotFoundError) Error() string {
	return "product not found"
}
