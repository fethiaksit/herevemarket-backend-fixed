package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
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
	ProductID string   `json:"productId" binding:"required"`
	Name      string   `json:"name"`
	Price     *float64 `json:"price"`
	Quantity  int      `json:"quantity" binding:"required"`
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

func (p *createOrderPaymentMethodRequest) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return nil
	}

	if strings.HasPrefix(trimmed, "\"") {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}

		p.ID = strings.TrimSpace(value)
		p.Label = strings.TrimSpace(value)
		return nil
	}

	type paymentMethodAlias createOrderPaymentMethodRequest
	var parsed paymentMethodAlias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}

	p.ID = strings.TrimSpace(parsed.ID)
	p.Label = strings.TrimSpace(parsed.Label)
	return nil
}

type createOrderRequest struct {
	Items         []createOrderItemRequest         `json:"items" binding:"required"`
	TotalPrice    float64                          `json:"totalPrice"`
	Customer      *createOrderCustomerRequest      `json:"customer" binding:"required"`
	PaymentMethod *createOrderPaymentMethodRequest `json:"paymentMethod" binding:"required"`
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
		rawBody, readErr := io.ReadAll(c.Request.Body)
		if readErr != nil {
			respondOrderError(c, http.StatusBadRequest, "Unable to read request body")
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(rawBody))
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("[ORDER] [DEBUG] invalid order payload: %s", strings.TrimSpace(string(rawBody)))
			respondOrderError(c, http.StatusBadRequest, "Invalid JSON format or missing required fields")
			return
		}
		log.Printf("[ORDER] [DEBUG] received order payload: %s", strings.TrimSpace(string(rawBody)))

		if validationErr := validateCreateOrderRequest(req); validationErr != nil {
			respondOrderError(c, http.StatusBadRequest, validationErr.Error())
			return
		}

		userID, err := userIDFromHeader(c.GetHeader("Authorization"), jwtSecret)
		if err != nil {
			log.Println("[ORDER] [ERROR] token validation failed:", err)
			respondOrderError(c, http.StatusUnauthorized, "unauthorized")
			return
		}

		resolvedItems, err := resolveOrderItems(c.Request.Context(), db, req.Items)
		if err != nil {
			var stockErr outOfStockError
			if errors.As(err, &stockErr) {
				respondOrderError(c, http.StatusBadRequest, "Stok yetersiz")
				return
			}
			var notFoundErr productNotFoundError
			if errors.As(err, &notFoundErr) {
				respondOrderError(c, http.StatusBadRequest, "Ürün bulunamadı")
				return
			}
			respondOrderError(c, http.StatusBadRequest, err.Error())
			return
		}

		order, err := buildOrderFromResolvedItems(req, resolvedItems)
		if err != nil {
			respondOrderError(c, http.StatusBadRequest, err.Error())
			return
		}
		order.UserID = userID

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		session, err := db.Client().StartSession()
		if err != nil {
			respondOrderError(c, http.StatusInternalServerError, "db error")
			return
		}
		defer session.EndSession(ctx)

		var orderID primitive.ObjectID
		_, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
			for _, item := range order.Items {
				var rawProduct bson.M
				err := db.Collection("products").FindOne(
					sessCtx,
					bson.M{
						"_id":       item.ProductID,
						"isDeleted": bson.M{"$ne": true},
					},
				).Decode(&rawProduct)
				if err == mongo.ErrNoDocuments {
					return nil, productNotFoundError{ProductID: item.ProductID}
				}
				if err != nil {
					return nil, err
				}

				product, err := normalizeProductDocument(rawProduct)
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
				respondOrderError(c, http.StatusBadRequest, "Stok yetersiz")
				return
			}
			var notFoundErr productNotFoundError
			if errors.As(err, &notFoundErr) {
				respondOrderError(c, http.StatusBadRequest, "Ürün bulunamadı")
				return
			}
			respondOrderError(c, http.StatusInternalServerError, "db error")
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

func resolveOrderItems(ctx context.Context, db *mongo.Database, reqItems []createOrderItemRequest) ([]models.OrderItem, error) {
	if len(reqItems) == 0 {
		return nil, errors.New("at least one item is required")
	}

	items := make([]models.OrderItem, 0, len(reqItems))
	for _, item := range reqItems {
		productID, err := primitive.ObjectIDFromHex(item.ProductID)
		if err != nil {
			return nil, errors.New("invalid productId")
		}

		if item.Quantity <= 0 {
			return nil, errors.New("quantity must be greater than zero")
		}

		var rawProduct bson.M
		err = db.Collection("products").FindOne(
			ctx,
			bson.M{"_id": productID, "isDeleted": bson.M{"$ne": true}},
		).Decode(&rawProduct)
		if err == mongo.ErrNoDocuments {
			return nil, productNotFoundError{ProductID: productID}
		}
		if err != nil {
			return nil, err
		}

		product, err := normalizeProductDocument(rawProduct)
		if err != nil {
			return nil, err
		}

		if product.Stock < item.Quantity {
			return nil, outOfStockError{ProductID: productID, Available: product.Stock, Requested: item.Quantity}
		}

		unitPrice := effectiveProductPrice(product.Price, product.SaleEnabled, product.SalePrice)
		items = append(items, models.OrderItem{
			ProductID: productID,
			Name:      strings.TrimSpace(product.Name),
			Price:     unitPrice,
			Quantity:  item.Quantity,
		})
	}

	return items, nil
}

func buildOrderFromResolvedItems(req createOrderRequest, items []models.OrderItem) (models.Order, error) {
	if req.PaymentMethod == nil {
		return models.Order{}, errors.New("paymentMethod is required")
	}
	if req.Customer == nil {
		return models.Order{}, errors.New("customer is required")
	}

	if req.PaymentMethod.ID != "cash" && req.PaymentMethod.ID != "card" {
		return models.Order{}, errors.New("invalid payment method")
	}

	var total float64
	for _, item := range items {
		total += item.Price * float64(item.Quantity)
	}

	return models.Order{
		Items:         items,
		TotalPrice:    total,
		Customer:      models.OrderCustomer(*req.Customer),
		PaymentMethod: req.PaymentMethod.ID,
		Status:        "pending",
		CreatedAt:     time.Now(),
	}, nil
}

func validateCreateOrderRequest(req createOrderRequest) error {
	if len(req.Items) == 0 {
		return errors.New("items array must not be empty")
	}

	for _, item := range req.Items {
		if strings.TrimSpace(item.ProductID) == "" {
			return errors.New("productId is required for each item")
		}
		if item.Quantity <= 0 {
			return errors.New("quantity must be greater than zero for each item")
		}
		if item.Price == nil || *item.Price <= 0 {
			return errors.New("price must be greater than zero for each item")
		}
	}

	if req.PaymentMethod == nil {
		return errors.New("paymentMethod is required")
	}
	if strings.TrimSpace(req.PaymentMethod.ID) == "" {
		return errors.New("paymentMethod id is required")
	}

	if req.Customer == nil {
		return errors.New("customer is required")
	}
	if strings.TrimSpace(req.Customer.Title) == "" {
		return errors.New("customer title is required")
	}
	if strings.TrimSpace(req.Customer.Detail) == "" {
		return errors.New("customer detail is required")
	}

	return nil
}

func respondOrderError(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"success": false,
		"message": message,
	})
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
