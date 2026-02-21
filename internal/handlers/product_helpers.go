package handlers

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"backend/internal/models"
)

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
		switch typed := val.(type) {
		case bool:
			raw["saleEnabled"] = typed
		case string:
			raw["saleEnabled"] = typed == "true"
		default:
			raw["saleEnabled"] = false
		}
	} else {
		raw["saleEnabled"] = false
	}

	if val, ok := raw["salePrice"]; ok {
		switch typed := val.(type) {
		case int32:
			raw["salePrice"] = float64(typed)
		case int64:
			raw["salePrice"] = float64(typed)
		case float64:
			raw["salePrice"] = typed
		case int:
			raw["salePrice"] = float64(typed)
		default:
			raw["salePrice"] = 0.0
		}
	} else {
		raw["salePrice"] = 0.0
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
