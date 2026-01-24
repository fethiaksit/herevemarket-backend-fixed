package handlers

import "github.com/gin-gonic/gin"

func Home() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Redirect(302, "/admin/login")
	}
}
