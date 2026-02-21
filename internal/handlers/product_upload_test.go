package handlers

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParseMultipartProductRequest_PicksLastSaleEnabledValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	_ = writer.WriteField("saleEnabled", "false")
	_ = writer.WriteField("saleEnabled", "true")
	_ = writer.WriteField("salePrice", "99")
	_ = writer.Close()

	req := httptest.NewRequest("PUT", "/admin/api/products/1", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	parsed, err := parseMultipartProductRequest(c)
	if err != nil {
		t.Fatalf("parseMultipartProductRequest returned error: %v", err)
	}
	if !parsed.SaleEnabledSet || !parsed.SaleEnabled {
		t.Fatalf("expected saleEnabled=true, got %+v", parsed)
	}
	if !parsed.SalePriceSet || parsed.SalePrice != 99 {
		t.Fatalf("expected salePrice=99, got %+v", parsed)
	}
}
