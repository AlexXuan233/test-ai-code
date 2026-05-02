package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "fraud-scorer/api/swagger"
	"fraud-scorer/internal/cache"
	"fraud-scorer/internal/config"
	"fraud-scorer/internal/handler"
	"fraud-scorer/internal/models"
	"fraud-scorer/internal/store"
	"fraud-scorer/internal/worker"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// @title           LuxeCart Fraud Scorer API
// @version         1.0
// @description     Real-time transaction risk scoring service for LuxeCart.
// @host            localhost:8080
// @BasePath        /
func main() {
	cfg := config.Load()

	// Initialize SQLite with GORM
	db, err := gorm.Open(sqlite.Open("fraud_scorer.db"), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	// Auto-migrate schema
	if err := db.AutoMigrate(&models.Transaction{}); err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}

	transactionStore := store.NewGORMStore(db)
	velocityCache := cache.NewVelocityCache(cfg.CacheTTL, cfg.CacheEviction)
	defer velocityCache.Stop()

	workerPool := worker.NewIngestWorkerPool(cfg.AsyncWriteChanSize, cfg.AsyncWriteWorkers, transactionStore)
	defer workerPool.Stop()

	h := handler.NewTransactionHandler(cfg, transactionStore, velocityCache, workerPool)

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.GET("/health", h.Health)
	r.POST("/api/v1/score", h.Score)
	r.POST("/api/v1/transactions", h.Ingest)
	r.GET("/api/v1/transactions", h.List)
	r.POST("/api/v1/feedback", h.Feedback)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("server forced to shutdown: %v", err)
		}
	}()

	log.Printf("fraud scorer running on http://localhost:%s", cfg.Port)
	log.Printf("swagger ui available at http://localhost:%s/swagger/index.html", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
}
