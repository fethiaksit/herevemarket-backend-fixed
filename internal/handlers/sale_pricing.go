package handlers

import "fmt"

type saleUpdateInput struct {
	Price       *float64
	SaleEnabled *bool
	SalePrice   *float64
}

type saleUpdateResult struct {
	Price          float64
	SaleEnabled    bool
	SalePrice      float64
	SetSaleEnabled bool
	SetSalePrice   bool
}

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

func resolveSaleUpdate(existingPrice float64, existingSaleEnabled bool, existingSalePrice float64, input saleUpdateInput) (saleUpdateResult, error) {
	result := saleUpdateResult{
		Price:       existingPrice,
		SaleEnabled: existingSaleEnabled,
		SalePrice:   existingSalePrice,
	}

	if input.Price != nil {
		result.Price = *input.Price
	}

	salePriceSetForValidation := existingSalePrice > 0

	if input.SaleEnabled != nil {
		result.SaleEnabled = *input.SaleEnabled
		result.SetSaleEnabled = true
		if !*input.SaleEnabled {
			result.SalePrice = 0
			result.SetSalePrice = true
			salePriceSetForValidation = false
		}
	}

	if input.SalePrice != nil {
		result.SalePrice = *input.SalePrice
		result.SetSalePrice = true
		salePriceSetForValidation = true
	}

	if err := validateSaleFields(result.Price, result.SaleEnabled, result.SalePrice, salePriceSetForValidation); err != nil {
		return saleUpdateResult{}, err
	}

	return result, nil
}
