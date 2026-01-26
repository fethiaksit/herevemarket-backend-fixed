package handlers

import (
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
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
	IsActive       bool
	IsActiveSet    bool
	IsCampaign     bool
	IsCampaignSet  bool
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
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return MultipartProductInput{}, err
		}
		input.Price = parsed
		input.PriceSet = true
	}

	if value, ok := c.GetPostForm("stock"); ok {
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return MultipartProductInput{}, err
		}
		input.Stock = parsed
		input.StockSet = true
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

	// ---- CATEGORY IDS (CRITICAL FIX) ----

	categoryIDs := c.PostFormArray("category_id")
	if len(categoryIDs) > 0 {
		input.CategoryIDs = categoryIDs
		input.CategoryIDSet = true
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
			!strings.Contains(err.Error(), "no such file") {
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

	dir := "/app/public/uploads/products"
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
	return filepath.ToSlash(filepath.Join("uploads", "products", filename)), nil
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

func respondMultipartError(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}
