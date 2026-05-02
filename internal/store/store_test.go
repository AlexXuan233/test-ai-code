package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"fraud-scorer/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	if err := db.AutoMigrate(&models.Transaction{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func TestGORMStore_CreateAndList(t *testing.T) {
	db := setupTestDB(t)
	s := NewGORMStore(db)
	ctx := context.Background()

	tx := &models.Transaction{
		TransactionID:   "TX-001",
		Timestamp:       time.Now(),
		CardHash:        "hash1",
		Amount:          5000,
		Currency:        "USD",
		CustomerEmail:   "test@example.com",
		ShippingAddress: "123 Main St",
		ShippingCountry: "US",
		BillingCountry:  "US",
		IPAddress:       "1.2.3.4",
		CustomerID:      "CUST-1",
		IsFirstTime:     false,
		CardBIN:         "424242",
		Score:           10,
		RiskLevel:       "LOW",
		Recommendation:  "approve",
	}

	if err := s.Create(ctx, tx); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	txs, err := s.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(txs) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(txs))
	}
	if txs[0].TransactionID != "TX-001" {
		t.Fatalf("expected TX-001, got %s", txs[0].TransactionID)
	}
}

func TestGORMStore_CountByCardHash(t *testing.T) {
	db := setupTestDB(t)
	s := NewGORMStore(db)
	ctx := context.Background()
	now := time.Now()

	for i := 0; i < 3; i++ {
		tx := &models.Transaction{
			TransactionID: fmt.Sprintf("TX-%d", i),
			Timestamp:     now.Add(-5 * time.Minute),
			CardHash:      "hash_card",
			Amount:        1000,
			Currency:      "USD",
		}
		if err := s.Create(ctx, tx); err != nil {
			t.Fatalf("create failed: %v", err)
		}
	}

	// One old transaction outside the window
	txOld := &models.Transaction{
		TransactionID: "TX-OLD",
		Timestamp:     now.Add(-2 * time.Hour),
		CardHash:      "hash_card",
		Amount:        1000,
		Currency:      "USD",
	}
	if err := s.Create(ctx, txOld); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	count, err := s.CountByCardHash(ctx, "hash_card", now.Add(-15*time.Minute))
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3, got %d", count)
	}
}

func TestGORMStore_AvgAmountByCustomer(t *testing.T) {
	db := setupTestDB(t)
	s := NewGORMStore(db)
	ctx := context.Background()

	amounts := []float64{3000, 5000, 7000}
	for i, amt := range amounts {
		tx := &models.Transaction{
			TransactionID: fmt.Sprintf("TX-%d", i),
			Timestamp:     time.Now(),
			CardHash:      "hash",
			Amount:        amt,
			Currency:      "USD",
			CustomerID:    "CUST-AVG",
		}
		if err := s.Create(ctx, tx); err != nil {
			t.Fatalf("create failed: %v", err)
		}
	}

	avg, err := s.AvgAmountByCustomer(ctx, "CUST-AVG")
	if err != nil {
		t.Fatalf("avg failed: %v", err)
	}
	expected := 5000.0
	if avg != expected {
		t.Fatalf("expected avg %f, got %f", expected, avg)
	}
}

func TestGORMStore_UpdateConfirmedStatus(t *testing.T) {
	db := setupTestDB(t)
	s := NewGORMStore(db)
	ctx := context.Background()

	tx := &models.Transaction{
		TransactionID:  "TX-FB",
		Timestamp:      time.Now(),
		CardHash:       "hash",
		Amount:         1000,
		Currency:       "USD",
		ConfirmedStatus: "",
	}
	if err := s.Create(ctx, tx); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	if err := s.UpdateConfirmedStatus(ctx, "TX-FB", "fraud"); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	txs, _ := s.List(ctx, 1, 0)
	if txs[0].ConfirmedStatus != "fraud" {
		t.Fatalf("expected confirmed_status fraud, got %s", txs[0].ConfirmedStatus)
	}
}
