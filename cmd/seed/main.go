package main

import (
	"fmt"
	"log"
	"os"

	"fraud-scorer/internal/models"
	"fraud-scorer/internal/seed"
	"fraud-scorer/internal/store"
	"fraud-scorer/internal/worker"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func main() {
	db, err := gorm.Open(sqlite.Open("fraud_scorer.db"), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}
	if err := db.AutoMigrate(&models.Transaction{}); err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}

	transactionStore := store.NewGORMStore(db)
	workerPool := worker.NewIngestWorkerPool(1000, 4, transactionStore)
	defer workerPool.Stop()

	if err := seed.Run(workerPool); err != nil {
		fmt.Println("seed failed:", err)
		os.Exit(1)
	}
}
