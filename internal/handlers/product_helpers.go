package handlers

import (
	"context"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"backend/internal/models"
)

func parseLooseBool(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		normalized := strings.TrimSpace(strings.ToLower(typed))
		return normalized == "true" || normalized == "1"
	case int:
		return typed == 1
	case int32:
		return typed == 1
	case int64:
		return typed == 1
	case float64:
		return typed == 1
	default:
		return false
	}
}

func parseLooseNumber(value interface{}) float64 {
	switch typed := value.(type) {
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	case float64:
		return typed
	case int:
		return float64(typed)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0.0
		}
		return parsed
	default:
		return 0.0
	}
}

func normalizeProductDocument(raw bson.M) (models.Product, error) {
	if cat, ok := raw["category"].(string); ok {
		raw["category"] = []string{cat}
	}

	if val, ok := raw["isCampaign"]; ok {
		switch typed := val.(type) {
		case string:
			raw["isCampaign"] = typed == "true"
		case bool:
			// already bool, keep as is
		default:
			raw["isCampaign"] = false
		}
	} else {
		raw["isCampaign"] = false
	}

	if val, ok := raw["stock"]; ok {
		switch typed := val.(type) {
		case int32:
			raw["stock"] = int(typed)
		case int64:
			raw["stock"] = int(typed)
		case float64:
			raw["stock"] = int(typed)
		case int:
			raw["stock"] = typed
		default:
			raw["stock"] = 0
		}
	} else {
		raw["stock"] = 0
	}

	if val, ok := raw["saleEnabled"]; ok {
		raw["saleEnabled"] = parseLooseBool(val)
	} else {
		raw["saleEnabled"] = false
	}

	if val, ok := raw["salePrice"]; ok {
		raw["salePrice"] = parseLooseNumber(val)
	} else {
		raw["salePrice"] = 0.0
	}

	if val, ok := raw["price"]; ok {
		raw["price"] = parseLooseNumber(val)
	} else {
		raw["price"] = 0.0
	}

	data, err := bson.Marshal(raw)
	if err != nil {
		return models.Product{}, err
	}

	var p models.Product
	if err := bson.Unmarshal(data, &p); err != nil {
		return models.Product{}, err
	}

	p.InStock = p.Stock > 0
	p.IsOnSale = isProductOnSale(p.Price, p.SaleEnabled, p.SalePrice)

	return p, nil
}

func decodeProducts(ctx context.Context, cursor *mongo.Cursor) ([]models.Product, error) {
	products := make([]models.Product, 0)

	for cursor.Next(ctx) {
		var raw bson.M
		if err := cursor.Decode(&raw); err != nil {
			return nil, err
		}

		product, err := normalizeProductDocument(raw)
		if err != nil {
			return nil, err
		}

		products = append(products, product)
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	return products, nil
}
