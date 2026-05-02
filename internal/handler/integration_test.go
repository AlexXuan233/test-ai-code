package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
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

func setupIntegrationRouter(t *testing.T) *gin.Engine {
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

	return r
}

func doRequest(t *testing.T, r *gin.Engine, method, path string, body interface{}) *httptest.ResponseRecorder {
	var reqBody []byte
	if body != nil {
		var err error
		reqBody, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, path, bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// Scenario 1: Normal loyal customer should be approved with zero flags
func TestIntegration_LoyalCustomer_LowRisk(t *testing.T) {
	r := setupIntegrationRouter(t)

	payload := models.ScoreRequest{
		TransactionID:   "TX-LOYAL-001",
		CardNumber:      "4242424242424242",
		Amount:          4500,
		Currency:        "AED",
		CustomerEmail:   "loyal@luxe.ae",
		ShippingAddress: "Palm Jumeirah Villa 12",
		ShippingCountry: "AE",
		BillingCountry:  "AE",
		IPAddress:       "192.168.10.20",
		CustomerID:      "CUST-LOYAL-1",
		IsFirstTime:     false,
		CardBIN:         "424242",
		DeviceID:        "DEV-IPHONE-1",
	}

	w := doRequest(t, r, "POST", "/api/v1/score", payload)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp models.RiskScoreResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if resp.Score != 0 {
		t.Fatalf("expected score 0, got %d", resp.Score)
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
}

// Scenario 2: Velocity attack — same card used rapidly should trigger review/decline
func TestIntegration_VelocityAttack(t *testing.T) {
	r := setupIntegrationRouter(t)
	// Use a prepaid BIN card so velocity + BIN pushes score into MEDIUM
	card := "5105123456789012"

	// Ingest 3 transactions with same card within the window
	for i := 1; i <= 3; i++ {
		payload := models.ScoreRequest{
			TransactionID:   fmt.Sprintf("TX-VEL-%d", i),
			CardNumber:      card,
			Amount:          3500,
			Currency:        "USD",
			CustomerEmail:   fmt.Sprintf("attacker%d@temp.com", i),
			ShippingAddress: "123 Scam St, Dubai",
			ShippingCountry: "AE",
			BillingCountry:  "AE",
			IPAddress:       "203.0.113.50",
			CustomerID:      fmt.Sprintf("CUST-ATTACK-%d", i),
			IsFirstTime:     true,
			CardBIN:         "510512",
			DeviceID:        "DEV-ATTACK",
		}
		w := doRequest(t, r, "POST", "/api/v1/transactions", payload)
		if w.Code != http.StatusCreated {
			t.Fatalf("ingest %d failed: %d %s", i, w.Code, w.Body.String())
		}
		time.Sleep(50 * time.Millisecond)
	}

	// 4th transaction scored in real-time should detect velocity
	payload := models.ScoreRequest{
		TransactionID:   "TX-VEL-004",
		CardNumber:      card,
		Amount:          3500,
		Currency:        "USD",
		CustomerEmail:   "attacker4@temp.com",
		ShippingAddress: "123 Scam St, Dubai",
		ShippingCountry: "AE",
		BillingCountry:  "AE",
		IPAddress:       "203.0.113.50",
		CustomerID:      "CUST-ATTACK-4",
		IsFirstTime:     true,
		CardBIN:         "510512",
		DeviceID:        "DEV-ATTACK",
	}

	w := doRequest(t, r, "POST", "/api/v1/score", payload)
	if w.Code != http.StatusOK {
		t.Fatalf("score failed: %d %s", w.Code, w.Body.String())
	}

	var resp models.RiskScoreResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !containsFlag(resp.Flags, "velocity_card_exceeded") {
		t.Fatalf("expected velocity_card_exceeded flag, got %v", resp.Flags)
	}
	if !containsFlag(resp.Flags, "prepaid_card_bin") {
		t.Fatalf("expected prepaid_card_bin flag, got %v", resp.Flags)
	}
	if resp.Score < 30 {
		t.Fatalf("expected elevated score for velocity attack, got %d", resp.Score)
	}
	if resp.Recommendation == "approve" {
		t.Fatalf("expected review or decline for velocity attack, got %s", resp.Recommendation)
	}
}

// Scenario 3: Shipping address scam — many different cards to same address
func TestIntegration_ShippingAddressScam(t *testing.T) {
	r := setupIntegrationRouter(t)
	address := "45 Luxury Ave, Dubai Marina"

	for i := 1; i <= 4; i++ {
		payload := models.ScoreRequest{
			TransactionID:   fmt.Sprintf("TX-SHIP-%d", i),
			CardNumber:      fmt.Sprintf("555555555555444%d", i),
			Amount:          8000,
			Currency:        "AED",
			CustomerEmail:   fmt.Sprintf("shipper%d@anon.com", i),
			ShippingAddress: address,
			ShippingCountry: "AE",
			BillingCountry:  "AE",
			IPAddress:       fmt.Sprintf("198.51.100.%d", i),
			CustomerID:      fmt.Sprintf("CUST-SHIP-%d", i),
			IsFirstTime:     true,
			CardBIN:         fmt.Sprintf("55555%d", i),
			DeviceID:        "DEV-SHIP",
		}
		w := doRequest(t, r, "POST", "/api/v1/transactions", payload)
		if w.Code != http.StatusCreated {
			t.Fatalf("ingest %d failed: %d %s", i, w.Code, w.Body.String())
		}
		time.Sleep(50 * time.Millisecond)
	}

	payload := models.ScoreRequest{
		TransactionID:   "TX-SHIP-005",
		CardNumber:      "5555555555554445",
		Amount:          8000,
		Currency:        "AED",
		CustomerEmail:   "shipper5@anon.com",
		ShippingAddress: address,
		ShippingCountry: "AE",
		BillingCountry:  "AE",
		IPAddress:       "198.51.100.5",
		CustomerID:      "CUST-SHIP-5",
		IsFirstTime:     true,
		CardBIN:         "555555",
		DeviceID:        "DEV-SHIP",
	}

	w := doRequest(t, r, "POST", "/api/v1/score", payload)
	if w.Code != http.StatusOK {
		t.Fatalf("score failed: %d %s", w.Code, w.Body.String())
	}

	var resp models.RiskScoreResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !containsFlag(resp.Flags, "velocity_shipping_exceeded") {
		t.Fatalf("expected velocity_shipping_exceeded flag, got %v", resp.Flags)
	}
}

// Scenario 4: Geographic mismatch — card billing Brazil, shipping Russia
func TestIntegration_GeographicMismatch(t *testing.T) {
	r := setupIntegrationRouter(t)

	payload := models.ScoreRequest{
		TransactionID:   "TX-GEO-001",
		CardNumber:      "4000000000000001",
		Amount:          8500,
		Currency:        "USD",
		CustomerEmail:   "global@shopper.com",
		ShippingAddress: "Moscow, Russia",
		ShippingCountry: "RU",
		BillingCountry:  "BR",
		IPAddress:       "45.22.11.10",
		CustomerID:      "CUST-GLOBAL",
		IsFirstTime:     false,
		CardBIN:         "400000",
		DeviceID:        "DEV-GLOBAL",
	}

	w := doRequest(t, r, "POST", "/api/v1/score", payload)
	if w.Code != http.StatusOK {
		t.Fatalf("score failed: %d %s", w.Code, w.Body.String())
	}

	var resp models.RiskScoreResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !containsFlag(resp.Flags, "country_mismatch") {
		t.Fatalf("expected country_mismatch flag, got %v", resp.Flags)
	}
	if resp.Score < 10 {
		t.Fatalf("expected score >= 10 for geo mismatch, got %d", resp.Score)
	}
}

// Scenario 5: High-value first-time purchase should raise flags
func TestIntegration_HighValueFirstPurchase(t *testing.T) {
	r := setupIntegrationRouter(t)

	payload := models.ScoreRequest{
		TransactionID:   "TX-HIGH-001",
		CardNumber:      "378282246310005",
		Amount:          28000,
		Currency:        "EUR",
		CustomerEmail:   "newrich@mail.com",
		ShippingAddress: "Champs Elysees, Paris",
		ShippingCountry: "FR",
		BillingCountry:  "FR",
		IPAddress:       "82.45.12.10",
		CustomerID:      "CUST-NEW-1",
		IsFirstTime:     true,
		CardBIN:         "378282",
		DeviceID:        "DEV-NEW",
	}

	w := doRequest(t, r, "POST", "/api/v1/score", payload)
	if w.Code != http.StatusOK {
		t.Fatalf("score failed: %d %s", w.Code, w.Body.String())
	}

	var resp models.RiskScoreResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !containsFlag(resp.Flags, "high_value_transaction") {
		t.Fatalf("expected high_value_transaction flag, got %v", resp.Flags)
	}
	if !containsFlag(resp.Flags, "high_value_first_purchase") {
		t.Fatalf("expected high_value_first_purchase flag, got %v", resp.Flags)
	}
	if resp.Score < 30 {
		t.Fatalf("expected elevated score for high-value first purchase, got %d", resp.Score)
	}
}

// Scenario 6: Known proxy/VPN IP should be flagged
func TestIntegration_KnownProxyIP(t *testing.T) {
	r := setupIntegrationRouter(t)

	payload := models.ScoreRequest{
		TransactionID:   "TX-PROXY-001",
		CardNumber:      "4000000000000002",
		Amount:          3000,
		Currency:        "AED",
		CustomerEmail:   "proxy@dark.net",
		ShippingAddress: "Dubai",
		ShippingCountry: "AE",
		BillingCountry:  "AE",
		IPAddress:       "10.0.0.50",
		CustomerID:      "CUST-PROXY",
		IsFirstTime:     true,
		CardBIN:         "400000",
		DeviceID:        "DEV-PROXY",
	}

	w := doRequest(t, r, "POST", "/api/v1/score", payload)
	if w.Code != http.StatusOK {
		t.Fatalf("score failed: %d %s", w.Code, w.Body.String())
	}

	var resp models.RiskScoreResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !containsFlag(resp.Flags, "known_proxy_vpn_ip") {
		t.Fatalf("expected known_proxy_vpn_ip flag, got %v", resp.Flags)
	}
}

// Scenario 7: Full lifecycle — ingest, score, list, feedback
func TestIntegration_FullLifecycle(t *testing.T) {
	r := setupIntegrationRouter(t)

	// Step 1: Ingest a transaction
	ingest := models.ScoreRequest{
		TransactionID:   "TX-LIFE-001",
		CardNumber:      "4242424242424242",
		Amount:          5000,
		Currency:        "USD",
		CustomerEmail:   "lifecycle@test.com",
		ShippingAddress: "Test Blvd",
		ShippingCountry: "US",
		BillingCountry:  "US",
		IPAddress:       "1.2.3.4",
		CustomerID:      "CUST-LIFE",
		IsFirstTime:     false,
		CardBIN:         "424242",
		DeviceID:        "DEV-LIFE",
	}
	w := doRequest(t, r, "POST", "/api/v1/transactions", ingest)
	if w.Code != http.StatusCreated {
		t.Fatalf("ingest failed: %d %s", w.Code, w.Body.String())
	}
	time.Sleep(100 * time.Millisecond)

	// Step 2: List transactions
	w = doRequest(t, r, "GET", "/api/v1/transactions?limit=10", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list failed: %d %s", w.Code, w.Body.String())
	}
	var txs []models.Transaction
	if err := json.Unmarshal(w.Body.Bytes(), &txs); err != nil {
		t.Fatalf("unmarshal list failed: %v", err)
	}
	if len(txs) != 1 {
		t.Fatalf("expected 1 transaction in list, got %d", len(txs))
	}

	// Step 3: Submit feedback
	fb := models.FeedbackRequest{
		TransactionID:   "TX-LIFE-001",
		ConfirmedStatus: "fraud",
	}
	w = doRequest(t, r, "POST", "/api/v1/feedback", fb)
	if w.Code != http.StatusOK {
		t.Fatalf("feedback failed: %d %s", w.Code, w.Body.String())
	}

	// Step 4: Score a new transaction (should work independently)
	scoreReq := models.ScoreRequest{
		TransactionID:   "TX-LIFE-002",
		CardNumber:      "4242424242424242",
		Amount:          2500,
		Currency:        "USD",
		CustomerEmail:   "lifecycle2@test.com",
		ShippingAddress: "Test Blvd 2",
		ShippingCountry: "US",
		BillingCountry:  "US",
		IPAddress:       "1.2.3.5",
		CustomerID:      "CUST-LIFE-2",
		IsFirstTime:     false,
		CardBIN:         "424242",
		DeviceID:        "DEV-LIFE-2",
	}
	w = doRequest(t, r, "POST", "/api/v1/score", scoreReq)
	if w.Code != http.StatusOK {
		t.Fatalf("score failed: %d %s", w.Code, w.Body.String())
	}
	var resp models.RiskScoreResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal score failed: %v", err)
	}
	if resp.RiskLevel != "LOW" {
		t.Fatalf("expected LOW for simple transaction, got %s", resp.RiskLevel)
	}
}

func containsFlag(flags []string, target string) bool {
	for _, f := range flags {
		if f == target {
			return true
		}
	}
	return false
}
