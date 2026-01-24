package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

func parsePaginationParams(pageStr, limitStr string) (int64, int64, error) {
	page := int64(1)
	limit := int64(20)

	if pageStr != "" {
		p, err := strconv.ParseInt(pageStr, 10, 64)
		if err != nil || p < 1 {
			return 0, 0, gin.Error{}
		}
		page = p
	}

	if limitStr != "" {
		l, err := strconv.ParseInt(limitStr, 10, 64)
		if err != nil || l < 1 {
			return 0, 0, gin.Error{}
		}
		limit = l
	}

	return page, limit, nil
}
