package handlers

import (
	"encoding/json"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

func TestValidateSaleFieldsMissingSalePrice(t *testing.T) {
	err := validateSaleFields(100, true, 0, false)
	if err == nil {
		t.Fatal("expected validation error when saleEnabled=true and salePrice is missing")
	}
}

func TestValidateSaleFieldsSalePriceGreaterOrEqualPrice(t *testing.T) {
	tests := []float64{100, 120}
	for _, salePrice := range tests {
		err := validateSaleFields(100, true, salePrice, true)
		if err == nil {
			t.Fatalf("expected validation error for salePrice=%v", salePrice)
		}
	}
}

func TestNormalizeProductDocumentIncludesSaleFields(t *testing.T) {
	product, err := normalizeProductDocument(bson.M{
		"name":        "Test",
		"price":       100.0,
		"saleEnabled": true,
		"salePrice":   80.0,
		"stock":       5,
		"category":    []string{"Cat"},
	})
	if err != nil {
		t.Fatalf("normalizeProductDocument returned error: %v", err)
	}
	if !product.SaleEnabled || product.SalePrice != 80 {
		t.Fatalf("expected sale fields to be preserved, got saleEnabled=%v salePrice=%v", product.SaleEnabled, product.SalePrice)
	}
	if !product.IsOnSale {
		t.Fatal("expected IsOnSale to be true")
	}
}

func TestProductJSONAlwaysIncludesSalePrice(t *testing.T) {
	product, err := normalizeProductDocument(bson.M{
		"name":        "Test",
		"price":       120.0,
		"saleEnabled": true,
		"salePrice":   99.0,
		"stock":       10,
		"category":    []string{"Meyve"},
	})
	if err != nil {
		t.Fatalf("normalizeProductDocument returned error: %v", err)
	}

	body, err := json.Marshal(product)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}

	jsonBody := string(body)
	if !strings.Contains(jsonBody, "\"salePrice\":99") {
		t.Fatalf("expected salePrice in response json, got %s", jsonBody)
	}
	if !strings.Contains(jsonBody, "\"isOnSale\":true") {
		t.Fatalf("expected isOnSale=true in response json, got %s", jsonBody)
	}
}

func TestEffectiveProductPriceUsesSalePriceWhenOnSale(t *testing.T) {
	if got := effectiveProductPrice(100, true, 75); got != 75 {
		t.Fatalf("expected sale price 75, got %v", got)
	}
	if got := effectiveProductPrice(100, false, 75); got != 100 {
		t.Fatalf("expected regular price 100 when sale disabled, got %v", got)
	}
}

func floatPtr(v float64) *float64 { return &v }
func boolPtr(v bool) *bool        { return &v }

func TestResolveSaleUpdate_EnableSaleRequiresSalePrice(t *testing.T) {
	_, err := resolveSaleUpdate(120, false, 0, saleUpdateInput{SaleEnabled: boolPtr(true)})
	if err == nil {
		t.Fatal("expected error when enabling sale without a salePrice")
	}
}

func TestResolveSaleUpdate_EnableSaleAndSetPrice(t *testing.T) {
	result, err := resolveSaleUpdate(120, false, 0, saleUpdateInput{SaleEnabled: boolPtr(true), SalePrice: floatPtr(99)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.SaleEnabled || result.SalePrice != 99 {
		t.Fatalf("expected saleEnabled=true and salePrice=99, got %+v", result)
	}
	if !result.SetSaleEnabled || !result.SetSalePrice {
		t.Fatalf("expected sale fields to be marked for update, got %+v", result)
	}
	if !isProductOnSale(result.Price, result.SaleEnabled, result.SalePrice) {
		t.Fatal("expected product to be on sale")
	}
}

func TestResolveSaleUpdate_ChangeSalePrice(t *testing.T) {
	result, err := resolveSaleUpdate(120, true, 99, saleUpdateInput{SalePrice: floatPtr(89)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SalePrice != 89 || !result.SetSalePrice {
		t.Fatalf("expected salePrice update to 89, got %+v", result)
	}
}

func TestResolveSaleUpdate_DisableSaleResetsSalePrice(t *testing.T) {
	result, err := resolveSaleUpdate(120, true, 99, saleUpdateInput{SaleEnabled: boolPtr(false)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SaleEnabled || result.SalePrice != 0 {
		t.Fatalf("expected sale disabled and salePrice reset, got %+v", result)
	}
	if !result.SetSaleEnabled || !result.SetSalePrice {
		t.Fatalf("expected saleEnabled/salePrice to be persisted on disable, got %+v", result)
	}
}

func TestResolveSaleUpdate_PartialUpdateDoesNotChangeSaleFields(t *testing.T) {
	result, err := resolveSaleUpdate(120, true, 99, saleUpdateInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.SaleEnabled || result.SalePrice != 99 {
		t.Fatalf("expected existing sale values to remain, got %+v", result)
	}
	if result.SetSaleEnabled || result.SetSalePrice {
		t.Fatalf("expected no sale fields to be included in update, got %+v", result)
	}
}
