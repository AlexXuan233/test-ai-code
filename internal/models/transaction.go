package models

import (
	"time"
)

// Transaction represents a stored transaction record.
type Transaction struct {
	ID                uint      `gorm:"primaryKey" json:"id" example:"1"`
	TransactionID     string    `gorm:"uniqueIndex" json:"transaction_id" example:"TX-DEMO-001"`
	Timestamp         time.Time `json:"timestamp"`
	CardHash          string    `json:"card_hash" example:"a1b2c3d4e5f67890"`
	Amount            float64   `json:"amount" example:"12500.00"`
	Currency          string    `json:"currency" example:"USD"`
	CustomerEmail     string    `json:"customer_email" example:"sarah.alrashed@email.ae"`
	ShippingAddress   string    `json:"shipping_address" example:"12 Palm Jumeirah, Dubai"`
	ShippingCountry   string    `json:"shipping_country" example:"AE"`
	BillingCountry    string    `json:"billing_country" example:"AE"`
	IPAddress         string    `json:"ip_address" example:"192.168.1.50"`
	CustomerID        string    `json:"customer_id" example:"CUST-001"`
	IsFirstTime       bool      `json:"is_first_time"`
	CardBIN           string    `json:"card_bin" example:"424242"`
	DeviceID          string    `json:"device_id" example:"DEV-001"`
	Score             int       `json:"score" example:"80"`
	RiskLevel         string    `json:"risk_level" example:"HIGH"`
	Flags             string    `json:"flags" example:"[\"high_value_transaction\",\"country_mismatch\"]"` // JSON array stored as string
	Recommendation    string    `json:"recommendation" example:"decline"`
	ConfirmedStatus   string    `json:"confirmed_status" example:"fraud"` // "", "fraud", "legitimate"
	CreatedAt         time.Time `json:"created_at"`
}

// ScoreRequest is the incoming payload for scoring a transaction.
type ScoreRequest struct {
	TransactionID   string  `json:"transaction_id" binding:"required" example:"TX-DEMO-001"`
	Timestamp       string  `json:"timestamp" example:"2026-05-02T10:00:00Z"`
	CardNumber      string  `json:"card_number" binding:"required" example:"4242424242424242"`
	Amount          float64 `json:"amount" binding:"required,gt=0" example:"12500.00"`
	Currency        string  `json:"currency" binding:"required" example:"USD"`
	CustomerEmail   string  `json:"customer_email" binding:"required,email" example:"sarah.alrashed@email.ae"`
	ShippingAddress string  `json:"shipping_address" binding:"required" example:"12 Palm Jumeirah, Dubai"`
	ShippingCountry string  `json:"shipping_country" binding:"required" example:"AE"`
	BillingCountry  string  `json:"billing_country" binding:"required" example:"AE"`
	IPAddress       string  `json:"ip_address" binding:"required" example:"192.168.1.50"`
	CustomerID      string  `json:"customer_id" example:"CUST-001"`
	IsFirstTime     bool    `json:"is_first_time" example:"false"`
	CardBIN         string  `json:"card_bin" example:"424242"`
	DeviceID        string  `json:"device_id" example:"DEV-001"`
}

// RiskScoreResponse is returned by the scoring endpoint.
type RiskScoreResponse struct {
	TransactionID  string   `json:"transaction_id" example:"TX-DEMO-001"`
	Score          int      `json:"score" example:"80"`
	RiskLevel      string   `json:"risk_level" example:"HIGH"`              // LOW, MEDIUM, HIGH, CRITICAL
	Flags          []string `json:"flags" example:"high_value_transaction,country_mismatch"` // detected risk factors
	Recommendation string   `json:"recommendation" example:"decline"`       // approve, review, decline
}

// FeedbackRequest allows marking a transaction as confirmed fraud or legitimate.
type FeedbackRequest struct {
	TransactionID   string `json:"transaction_id" binding:"required" example:"TX-DEMO-001"`
	ConfirmedStatus string `json:"confirmed_status" binding:"required,oneof=fraud legitimate" example:"fraud"`
}

// ErrorResponse is returned for 4xx/5xx errors.
type ErrorResponse struct {
	Error string `json:"error" example:"invalid request: Field 'amount' is required"`
}

// IngestResponse is returned after ingesting a transaction.
type IngestResponse struct {
	Status        string `json:"status" example:"ingested"`
	TransactionID string `json:"transaction_id" example:"TX-DEMO-001"`
}

// FeedbackResponse is returned after submitting feedback.
type FeedbackResponse struct {
	Status          string `json:"status" example:"updated"`
	TransactionID   string `json:"transaction_id" example:"TX-DEMO-001"`
	ConfirmedStatus string `json:"confirmed_status" example:"fraud"`
}

// HealthResponse is returned by the health check endpoint.
type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}
