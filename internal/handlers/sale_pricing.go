package handlers

import "fmt"

func isProductOnSale(price float64, saleEnabled bool, salePrice float64) bool {
	return saleEnabled && salePrice > 0 && salePrice < price
}

func effectiveProductPrice(price float64, saleEnabled bool, salePrice float64) float64 {
	if isProductOnSale(price, saleEnabled, salePrice) {
		return salePrice
	}
	return price
}

func validateSaleFields(price float64, saleEnabled bool, salePrice float64, salePriceSet bool) error {
	if !saleEnabled {
		return nil
	}
	if !salePriceSet {
		return fmt.Errorf("salePrice is required when saleEnabled is true")
	}
	if salePrice <= 0 {
		return fmt.Errorf("salePrice must be greater than 0")
	}
	if salePrice >= price {
		return fmt.Errorf("salePrice must be less than price")
	}
	return nil
}
