package main

import (
    "log"
    "net/http"
    "os"
    "time"

    "github.com/gorilla/mux"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
    "gorm.io/gorm/logger"

    "online-store/internal/application"
    "online-store/internal/domain"
    "online-store/internal/infrastructure"
    "online-store/internal/transport"
)

func main() {

    dsn := os.Getenv("DATABASE_URL")
    if dsn == "" {
        dsn = "host=db user=postgres password=postgres dbname=store port=5432 sslmode=disable"
    }

    db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
        Logger: logger.Default.LogMode(logger.Info),
    })
    if err != nil {
        log.Fatal("Failed to connect to database:", err)
    }

    err = db.AutoMigrate(
        &domain.Customer{},
        &domain.Product{},
        &domain.Order{},
        &domain.OrderItem{},
    )
    if err != nil {
        log.Fatal("Failed to migrate database:", err)
    }


    custRepo := infrastructure.NewCustomerRepository(db)
    prodRepo := infrastructure.NewProductRepository(db)
    orderRepo := infrastructure.NewOrderRepository(db)


    custService := application.NewCustomerService(custRepo)
    prodService := application.NewProductService(prodRepo)
    orderService := application.NewOrderService(orderRepo, prodRepo, custRepo)


    handler := transport.NewHandler(orderService, custService, prodService)
    router := mux.NewRouter()
    handler.RegisterRoutes(router)

    srv := &http.Server{
        Handler:      router,
        Addr:         ":8080",
        WriteTimeout: 15 * time.Second,
        ReadTimeout:  15 * time.Second,
    }

    log.Println("Server started on :8080")
    log.Fatal(srv.ListenAndServe())
}