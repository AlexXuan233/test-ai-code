package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fraud-scorer/internal/cache"
	"fraud-scorer/internal/config"
	"fraud-scorer/internal/models"
	"fraud-scorer/internal/store"
	"fraud-scorer/internal/worker"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupHandler(t *testing.T) (*gin.Engine, *TransactionHandler) {
	gin.SetMode(gin.TestMode)
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
	workers := worker.NewIngestWorkerPool(100, 2, s)
	t.Cleanup(workers.Stop)

	h := NewTransactionHandler(cfg, s, c, workers)
	r := gin.New()
	r.POST("/api/v1/score", h.Score)
	r.POST("/api/v1/transactions", h.Ingest)
	r.GET("/api/v1/transactions", h.List)
	r.POST("/api/v1/feedback", h.Feedback)
	r.GET("/health", h.Health)

	return r, h
}

func TestScoreEndpoint(t *testing.T) {
	r, _ := setupHandler(t)

	reqBody := models.ScoreRequest{
		TransactionID:   "TX-TEST-001",
		CardNumber:      "4242424242424242",
		Amount:          3500,
		Currency:        "AED",
		CustomerEmail:   "test@example.com",
		ShippingAddress: "Dubai",
		ShippingCountry: "AE",
		BillingCountry:  "AE",
		IPAddress:       "192.168.1.1",
		CustomerID:      "CUST-TEST",
		IsFirstTime:     false,
		CardBIN:         "424242",
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/score", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp models.RiskScoreResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if resp.RiskLevel != "LOW" {
		t.Fatalf("expected LOW, got %s", resp.RiskLevel)
	}
}

func TestScoreEndpoint_InvalidRequest(t *testing.T) {
	r, _ := setupHandler(t)

	reqBody := map[string]interface{}{
		"transaction_id": "TX-BAD",
		// missing required fields
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/score", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestIngestEndpoint(t *testing.T) {
	r, _ := setupHandler(t)

	reqBody := models.ScoreRequest{
		TransactionID:   "TX-INGEST-001",
		CardNumber:      "4242424242424242",
		Amount:          5000,
		Currency:        "USD",
		CustomerEmail:   "ingest@test.com",
		ShippingAddress: "Test Ave",
		ShippingCountry: "US",
		BillingCountry:  "US",
		IPAddress:       "1.2.3.4",
		CustomerID:      "CUST-INGEST",
		IsFirstTime:     false,
		CardBIN:         "424242",
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/transactions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListEndpoint(t *testing.T) {
	r, _ := setupHandler(t)

	// Seed via ingest
	reqBody := models.ScoreRequest{
		TransactionID:   "TX-LIST-001",
		CardNumber:      "4242424242424242",
		Amount:          5000,
		Currency:        "USD",
		CustomerEmail:   "list@test.com",
		ShippingAddress: "List Ave",
		ShippingCountry: "US",
		BillingCountry:  "US",
		IPAddress:       "1.2.3.4",
		CustomerID:      "CUST-LIST",
		IsFirstTime:     false,
		CardBIN:         "424242",
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/transactions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// Allow async worker to persist
	time.Sleep(100 * time.Millisecond)

	// Now list
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/transactions?limit=10", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var txs []models.Transaction
	if err := json.Unmarshal(w.Body.Bytes(), &txs); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(txs) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(txs))
	}
}

func TestFeedbackEndpoint(t *testing.T) {
	r, _ := setupHandler(t)

	// Seed a transaction first
	reqBody := models.ScoreRequest{
		TransactionID:   "TX-FB-001",
		CardNumber:      "4242424242424242",
		Amount:          5000,
		Currency:        "USD",
		CustomerEmail:   "fb@test.com",
		ShippingAddress: "FB Ave",
		ShippingCountry: "US",
		BillingCountry:  "US",
		IPAddress:       "1.2.3.4",
		CustomerID:      "CUST-FB",
		IsFirstTime:     false,
		CardBIN:         "424242",
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/transactions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// Submit feedback
	fb := models.FeedbackRequest{
		TransactionID:   "TX-FB-001",
		ConfirmedStatus: "fraud",
	}
	jsonBody, _ = json.Marshal(fb)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/feedback", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHealthEndpoint(t *testing.T) {
	r, _ := setupHandler(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("ok")) {
		t.Fatalf("expected ok in body, got %s", w.Body.String())
	}
}
