package scorer

import (
	"context"
	"fmt"
	"testing"
	"time"

	"fraud-scorer/internal/cache"
	"fraud-scorer/internal/config"
	"fraud-scorer/internal/models"
	"fraud-scorer/internal/store"
	"fraud-scorer/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupScorer(t *testing.T) (*RiskScorer, store.TransactionStore) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	if err := db.AutoMigrate(&models.Transaction{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	s := store.NewGORMStore(db)
	cfg := config.Load()
	c := cache.NewVelocityCache(time.Minute, time.Minute)
	t.Cleanup(c.Stop)

	return NewRiskScorer(cfg, s, c), s
}

func TestScore_LowRisk(t *testing.T) {
	scorer, _ := setupScorer(t)
	ctx := context.Background()

	req := models.ScoreRequest{
		TransactionID:   "TX-LOW-001",
		CardNumber:      "4242424242424242",
		Amount:          3500,
		Currency:        "AED",
		CustomerEmail:   "loyal@customer.ae",
		ShippingAddress: "Dubai Marina",
		ShippingCountry: "AE",
		BillingCountry:  "AE",
		IPAddress:       "192.168.1.10",
		CustomerID:      "CUST-LOYAL",
		IsFirstTime:     false,
		CardBIN:         "424242",
	}

	resp, tx, err := scorer.Score(ctx, req)
	if err != nil {
		t.Fatalf("score failed: %v", err)
	}

	if resp.Score != 0 {
		t.Fatalf("expected score 0 for low-risk, got %d", resp.Score)
	}
	if resp.RiskLevel != "LOW" {
		t.Fatalf("expected LOW, got %s", resp.RiskLevel)
	}
	if resp.Recommendation != "approve" {
		t.Fatalf("expected approve, got %s", resp.Recommendation)
	}
	if len(resp.Flags) != 0 {
		t.Fatalf("expected no flags, got %v", resp.Flags)
	}
	if tx.TransactionID != req.TransactionID {
		t.Fatal("transaction mismatch")
	}
}

func TestScore_HighValueFirstPurchase(t *testing.T) {
	scorer, _ := setupScorer(t)
	ctx := context.Background()

	req := models.ScoreRequest{
		TransactionID:   "TX-HIGH-001",
		CardNumber:      "4111111111111111",
		Amount:          25000,
		Currency:        "USD",
		CustomerEmail:   "new@customer.com",
		ShippingAddress: "New Address",
		ShippingCountry: "US",
		BillingCountry:  "US",
		IPAddress:       "203.0.113.10",
		CustomerID:      "CUST-NEW",
		IsFirstTime:     true,
		CardBIN:         "411111",
	}

	resp, _, err := scorer.Score(ctx, req)
	if err != nil {
		t.Fatalf("score failed: %v", err)
	}

	if resp.Score < 30 {
		t.Fatalf("expected elevated score for high-value first purchase, got %d", resp.Score)
	}
	if !contains(resp.Flags, "high_value_transaction") {
		t.Fatalf("expected high_value_transaction flag, got %v", resp.Flags)
	}
	if !contains(resp.Flags, "high_value_first_purchase") {
		t.Fatalf("expected high_value_first_purchase flag, got %v", resp.Flags)
	}
}

func TestScore_CountryMismatch(t *testing.T) {
	scorer, _ := setupScorer(t)
	ctx := context.Background()

	req := models.ScoreRequest{
		TransactionID:   "TX-GEO-001",
		CardNumber:      "5555555555554444",
		Amount:          5000,
		Currency:        "USD",
		CustomerEmail:   "geo@test.com",
		ShippingAddress: "Moscow",
		ShippingCountry: "RU",
		BillingCountry:  "BR",
		IPAddress:       "45.22.11.10",
		CustomerID:      "CUST-GEO",
		IsFirstTime:     false,
		CardBIN:         "555555",
	}

	resp, _, err := scorer.Score(ctx, req)
	if err != nil {
		t.Fatalf("score failed: %v", err)
	}

	if !contains(resp.Flags, "country_mismatch") {
		t.Fatalf("expected country_mismatch flag, got %v", resp.Flags)
	}
}

func TestScore_KnownProxyIP(t *testing.T) {
	scorer, _ := setupScorer(t)
	ctx := context.Background()

	req := models.ScoreRequest{
		TransactionID:   "TX-PROXY-001",
		CardNumber:      "4000000000000001",
		Amount:          3000,
		Currency:        "AED",
		CustomerEmail:   "proxy@test.com",
		ShippingAddress: "Dubai",
		ShippingCountry: "AE",
		BillingCountry:  "AE",
		IPAddress:       "10.0.0.50",
		CustomerID:      "CUST-PROXY",
		IsFirstTime:     true,
		CardBIN:         "400000",
	}

	resp, _, err := scorer.Score(ctx, req)
	if err != nil {
		t.Fatalf("score failed: %v", err)
	}

	if !contains(resp.Flags, "known_proxy_vpn_ip") {
		t.Fatalf("expected known_proxy_vpn_ip flag, got %v", resp.Flags)
	}
}

func TestScore_VelocityCard(t *testing.T) {
	scorer, s := setupScorer(t)
	ctx := context.Background()

	card := "4111111111111111"
	now := time.Now()
	// Seed 3 recent transactions with same card
	for i := 0; i < 3; i++ {
		tx := &models.Transaction{
			TransactionID:   fmt.Sprintf("TX-VEL-%d", i),
			Timestamp:       now.Add(-5 * time.Minute),
			CardHash:        utils.HashCard(card),
			Amount:          2000,
			Currency:        "USD",
			CustomerEmail:   "vel@test.com",
			ShippingAddress: "Vel Ave",
			ShippingCountry: "US",
			BillingCountry:  "US",
			IPAddress:       "203.0.113.10",
			CustomerID:      "CUST-VEL",
			IsFirstTime:     true,
			CardBIN:         "411111",
		}
		if err := s.Create(ctx, tx); err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}

	req := models.ScoreRequest{
		TransactionID:   "TX-VEL-004",
		CardNumber:      card,
		Amount:          2000,
		Currency:        "USD",
		CustomerEmail:   "vel@test.com",
		ShippingAddress: "Vel Ave",
		ShippingCountry: "US",
		BillingCountry:  "US",
		IPAddress:       "203.0.113.10",
		CustomerID:      "CUST-VEL",
		IsFirstTime:     true,
		CardBIN:         "411111",
	}

	resp, _, err := scorer.Score(ctx, req)
	if err != nil {
		t.Fatalf("score failed: %v", err)
	}

	if !contains(resp.Flags, "velocity_card_exceeded") {
		t.Fatalf("expected velocity_card_exceeded flag, got %v", resp.Flags)
	}
}

func TestScore_BINRisk(t *testing.T) {
	scorer, _ := setupScorer(t)
	ctx := context.Background()

	req := models.ScoreRequest{
		TransactionID:   "TX-BIN-001",
		CardNumber:      "5105999999999999",
		Amount:          3000,
		Currency:        "USD",
		CustomerEmail:   "bin@test.com",
		ShippingAddress: "Test",
		ShippingCountry: "US",
		BillingCountry:  "US",
		IPAddress:       "1.2.3.4",
		CustomerID:      "CUST-BIN",
		IsFirstTime:     false,
		CardBIN:         "510599",
	}

	resp, _, err := scorer.Score(ctx, req)
	if err != nil {
		t.Fatalf("score failed: %v", err)
	}

	if !contains(resp.Flags, "prepaid_card_bin") {
		t.Fatalf("expected prepaid_card_bin flag, got %v", resp.Flags)
	}
}

func TestScore_Clamping(t *testing.T) {
	scorer, _ := setupScorer(t)
	ctx := context.Background()

	// A request that would theoretically exceed 100 points
	req := models.ScoreRequest{
		TransactionID:   "TX-MAX-001",
		CardNumber:      "5105999999999999",
		Amount:          50000, // extreme high value
		Currency:        "USD",
		CustomerEmail:   "max@test.com",
		ShippingAddress: "Test",
		ShippingCountry: "RU",
		BillingCountry:  "BR", // country mismatch
		IPAddress:       "10.0.0.50", // proxy
		CustomerID:      "CUST-MAX",
		IsFirstTime:     true, // first time + high value
		CardBIN:         "510599", // prepaid
	}

	resp, _, err := scorer.Score(ctx, req)
	if err != nil {
		t.Fatalf("score failed: %v", err)
	}

	if resp.Score > 100 {
		t.Fatalf("score should be clamped to max 100, got %d", resp.Score)
	}
	if resp.Score < 0 {
		t.Fatalf("score should be clamped to min 0, got %d", resp.Score)
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}


