package seed

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"fraud-scorer/internal/models"
	"fraud-scorer/internal/worker"
)

// Run reads testdata/transactions.json and bulk-ingests via the worker pool.
func Run(workerPool *worker.IngestWorkerPool) error {
	data, err := os.ReadFile("testdata/transactions.json")
	if err != nil {
		return fmt.Errorf("read testdata: %w", err)
	}

	var reqs []models.ScoreRequest
	if err := json.Unmarshal(data, &reqs); err != nil {
		return fmt.Errorf("unmarshal testdata: %w", err)
	}

	for _, req := range reqs {
		cardHash := hashCard(req.CardNumber)
		ts, _ := time.Parse(time.RFC3339, req.Timestamp)
		if req.Timestamp == "" {
			ts = time.Now()
		}

		flags := []string{}
		flagsJSON, _ := json.Marshal(flags)

		tx := &models.Transaction{
			TransactionID:   req.TransactionID,
			Timestamp:       ts,
			CardHash:        cardHash,
			Amount:          req.Amount,
			Currency:        req.Currency,
			CustomerEmail:   req.CustomerEmail,
			ShippingAddress: req.ShippingAddress,
			ShippingCountry: req.ShippingCountry,
			BillingCountry:  req.BillingCountry,
			IPAddress:       req.IPAddress,
			CustomerID:      req.CustomerID,
			IsFirstTime:     req.IsFirstTime,
			CardBIN:         req.CardBIN,
			DeviceID:        req.DeviceID,
			Score:           0,
			RiskLevel:       "LOW",
			Flags:           string(flagsJSON),
			Recommendation:  "approve",
		}
		workerPool.QueueBlocking(tx)
	}

	// Allow workers to flush
	time.Sleep(2 * time.Second)
	fmt.Printf("seeded %d transactions\n", len(reqs))
	return nil
}

func hashCard(cardNumber string) string {
	h := 0
	for _, c := range cardNumber {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return fmt.Sprintf("%d", h%100000000)
}
