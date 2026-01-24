package handlers

import "github.com/gin-gonic/gin"

func AdminLoginPage(c *gin.Context) {
	c.HTML(200, "login.html", gin.H{})
}

func AdminCategoriesPage(c *gin.Context) {
	c.HTML(200, "categories.html", gin.H{})
}

func AdminProductsPage(c *gin.Context) {
	c.HTML(200, "products.html", gin.H{})
}

func AdminOrdersPage(c *gin.Context) {
	c.HTML(200, "orders.html", gin.H{})
}
