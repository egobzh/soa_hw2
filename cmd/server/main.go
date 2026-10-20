package main

import (
	"log"

	_ "marketplace/docs"
	"marketplace/internal/config"
	deliveryHttp "marketplace/internal/delivery/http"
	"marketplace/internal/repository/postgres"
	"marketplace/internal/repository/postgres/operation"
	"marketplace/internal/repository/postgres/order"
	"marketplace/internal/repository/postgres/product"
	"marketplace/internal/repository/postgres/promocode"
	"marketplace/internal/service"
)

// @title Marketplace Service API
// @version 1.0
// @description Сервис маркетплейса с контрактным подходом OpenAPI, CRUD товаров, сложной бизнес-логикой заказов, промокодами и JSON-логированием (ДЗ №2, 8 баллов).
// @host localhost:8080
// @BasePath /
func main() {
	cfg := config.LoadConfig()

	log.Printf("Connecting to database...")
	pg, err := postgres.NewDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Database connection error: %v", err)
	}
	defer pg.SQL.Close()
	log.Printf("Database connected and schema initialized.")

	// Репозитории
	productRepo := product.NewRepository(pg.SQL)
	orderRepo := order.NewRepository(pg.SQL)
	promoRepo := promocode.NewRepository(pg.SQL)
	opRepo := operation.NewRepository(pg.SQL)

	// Сервисы
	productService := service.NewProductService(productRepo)
	orderService := service.NewOrderService(orderRepo, productRepo, promoRepo, opRepo, cfg.OrderRateLimitMinutes)
	promoService := service.NewPromoCodeService(promoRepo)

	// Хэндлеры и роутер
	handler := deliveryHttp.NewHandler(productService, orderService, promoService)
	router := deliveryHttp.SetupRouter(handler)

	log.Printf("Starting Marketplace HTTP Server on port :%s", cfg.ServerPort)
	log.Printf("Swagger UI available at http://localhost:%s/swagger/index.html", cfg.ServerPort)

	if err := router.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
