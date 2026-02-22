package handlers

import (
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

/*
=======================
  INPUT STRUCT
=======================
*/

type MultipartProductInput struct {
	Name           string
	NameSet        bool
	Price          float64
	PriceSet       bool
	CategoryIDs    []string
	CategoryIDSet  bool
	Description    string
	DescriptionSet bool
	Barcode        string
	BarcodeSet     bool
	Brand          string
	BrandSet       bool
	ImagePath      string
	ImageSet       bool
	Stock          int
	StockSet       bool
	InStock        bool
	InStockSet     bool
	IsActive       bool
	IsActiveSet    bool
	IsCampaign     bool
	IsCampaignSet  bool

	SaleEnabled    bool
	SaleEnabledSet bool
	SalePrice      float64
	SalePriceSet   bool
}

/*
=======================
  PARSER
=======================
*/

func parseMultipartProductRequest(c *gin.Context) (MultipartProductInput, error) {
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		log.Println("PARSE ERROR:", err)
		return MultipartProductInput{}, err
	}

	input := MultipartProductInput{}

	// ---- STRING FIELDS ----

	if value, ok := c.GetPostForm("name"); ok {
		input.Name = strings.TrimSpace(value)
		input.NameSet = true
	}

	if value, ok := c.GetPostForm("description"); ok {
		input.Description = strings.TrimSpace(value)
		input.DescriptionSet = true
	}

	if value, ok := c.GetPostForm("barcode"); ok {
		input.Barcode = strings.TrimSpace(value)
		input.BarcodeSet = true
	}

	if value, ok := c.GetPostForm("brand"); ok {
		input.Brand = strings.TrimSpace(value)
		input.BrandSet = true
	}

	// ---- NUMBER FIELDS ----

	if value, ok := c.GetPostForm("price"); ok {
		v := strings.TrimSpace(value)
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return MultipartProductInput{}, err
		}
		input.Price = parsed
		input.PriceSet = true
	}

	if value, ok := c.GetPostForm("stock"); ok {
		v := strings.TrimSpace(value)
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return MultipartProductInput{}, err
		}
		input.Stock = parsed
		input.StockSet = true
	}

	// ✅ salePrice: boş gelebilir -> 0 kabul et
	if value, ok := getPostFormAny(c, "salePrice", "sale_price"); ok {
		v := strings.TrimSpace(value)
		if v == "" {
			input.SalePrice = 0
			input.SalePriceSet = true
		} else {
			parsed, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return MultipartProductInput{}, err
			}
			input.SalePrice = parsed
			input.SalePriceSet = true
		}
	}

	// ---- BOOL FIELDS ----

	if value, ok := c.GetPostForm("isActive"); ok {
		parsed, err := parseBoolValue(value)
		if err != nil {
			return MultipartProductInput{}, err
		}
		input.IsActive = parsed
		input.IsActiveSet = true
	}

	if value, ok := c.GetPostForm("isCampaign"); ok {
		parsed, err := parseBoolValue(value)
		if err != nil {
			return MultipartProductInput{}, err
		}
		input.IsCampaign = parsed
		input.IsCampaignSet = true
	}

	if value, ok := c.GetPostForm("inStock"); ok {
		parsed, err := parseBoolValue(value)
		if err != nil {
			return MultipartProductInput{}, err
		}
		input.InStock = parsed
		input.InStockSet = true
	}

	if value, ok := getPostFormAny(c, "saleEnabled", "sale_enabled"); ok {
		parsed, err := parseBoolValue(value)
		if err != nil {
			return MultipartProductInput{}, err
		}
		input.SaleEnabled = parsed
		input.SaleEnabledSet = true
	}

	// ---- CATEGORY IDS ----
	categoryIDs := c.PostFormArray("category_id")
	if len(categoryIDs) > 0 {
		// trim + boşları at
		clean := make([]string, 0, len(categoryIDs))
		for _, v := range categoryIDs {
			vv := strings.TrimSpace(v)
			if vv != "" {
				clean = append(clean, vv)
			}
		}
		if len(clean) > 0 {
			input.CategoryIDs = clean
			input.CategoryIDSet = true
		}
	}

	// ---- IMAGE FILE ----
	file, err := c.FormFile("image")
	if err == nil {
		imagePath, err := saveImage(file)
		if err != nil {
			return MultipartProductInput{}, err
		}
		input.ImagePath = imagePath
		input.ImageSet = true
	} else {
		// toleranslı hata kontrolü (Gin sürümleri farkı)
		if !errors.Is(err, http.ErrMissingFile) &&
			!strings.Contains(strings.ToLower(err.Error()), "no such file") &&
			!strings.Contains(strings.ToLower(err.Error()), "missing file") {
			return MultipartProductInput{}, err
		}
	}

	return input, nil
}

/*
=======================
  IMAGE SAVE
=======================
*/

func saveImage(file *multipart.FileHeader) (string, error) {
	extension := strings.ToLower(filepath.Ext(file.Filename))
	if extension == "" {
		return "", fmt.Errorf("image file extension is required")
	}
	allowedExtensions := map[string]struct{}{
		".jpg":  {},
		".jpeg": {},
		".png":  {},
		".webp": {},
	}
	if _, ok := allowedExtensions[extension]; !ok {
		return "", fmt.Errorf("unsupported image type: %s", extension)
	}
	const maxImageSize = 5 << 20
	if file.Size > maxImageSize {
		return "", fmt.Errorf("image file too large (max 5MB)")
	}

	filename := primitive.NewObjectID().Hex() + extension

	dir := filepath.Join(publicRootDir, "uploads", "products")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("[UPLOAD] saveImage: failed to create directory %s: %v", dir, err)
		return "", err
	}

	fullPath := filepath.Join(dir, filename)
	log.Printf("[UPLOAD] saveImage: filename=%s ext=%s fullPath=%s", filename, extension, fullPath)

	out, err := os.Create(fullPath)
	if err != nil {
		log.Printf("[UPLOAD] saveImage: failed to create file %s: %v", fullPath, err)
		return "", err
	}
	defer out.Close()

	in, err := file.Open()
	if err != nil {
		log.Printf("[UPLOAD] saveImage: failed to open upload %s: %v", file.Filename, err)
		return "", err
	}
	defer in.Close()

	if _, err := io.Copy(out, in); err != nil {
		log.Printf("[UPLOAD] saveImage: failed to save file %s: %v", fullPath, err)
		return "", err
	}

	// DB’ye yazılacak path
	return path.Join("uploads", "products", filename), nil
}

/*
=======================
  HELPERS
=======================
*/

func parseBoolValue(value string) (bool, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "on" {
		return true, nil
	}
	return strconv.ParseBool(value)
}

// Çoklu aynı key gelirse en sondaki anlamlı değeri alır.
// Not: bazı durumlarda "" döndürebilir ama ok=true olur (hidden input yüzünden).
func getPostFormAny(c *gin.Context, keys ...string) (string, bool) {
	for _, key := range keys {
		values := c.PostFormArray(key)
		if len(values) == 0 {
			continue
		}
		for i := len(values) - 1; i >= 0; i-- {
			value := strings.TrimSpace(values[i])
			if value == "" {
				continue
			}
			return value, true
		}
		// key vardı ama hepsi boştu
		return "", true
	}
	return "", false
}

func respondMultipartError(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}
