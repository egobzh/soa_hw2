package http

import (
	"marketplace/internal/delivery/http/middleware"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func SetupRouter(handler *Handler) *gin.Engine {
	r := gin.New()

	r.Use(middleware.JSONLoggingMiddleware())
	r.Use(gin.Recovery())

	// Swagger документация
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Healthcheck
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Эндпоинты товаров (CRUD)
	products := r.Group("/products")
	{
		products.POST("", handler.CreateProduct)
		products.GET("", handler.GetProducts)
		products.GET("/:id", handler.GetProductByID)
		products.PUT("/:id", handler.UpdateProduct)
		products.DELETE("/:id", handler.DeleteProduct)
	}

	// Эндпоинты заказов (Business Logic)
	orders := r.Group("/orders")
	{
		orders.POST("", handler.CreateOrder)
		orders.PUT("/:id", handler.UpdateOrder)
		orders.POST("/:id/cancel", handler.CancelOrder)
	}

	// Эндпоинты промокодов
	promoCodes := r.Group("/promo-codes")
	{
		promoCodes.POST("", handler.CreatePromoCode)
	}

	return r
}
