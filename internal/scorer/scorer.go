package scorer

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"time"

	"fraud-scorer/internal/cache"
	"fraud-scorer/internal/config"
	"fraud-scorer/internal/models"
	"fraud-scorer/internal/store"
	"fraud-scorer/pkg/utils"
)

// RiskScorer evaluates transactions using a rule-based weighted engine.
type RiskScorer struct {
	cfg   *config.Config
	store store.TransactionStore
	cache *cache.VelocityCache
}

// NewRiskScorer creates a new scorer.
func NewRiskScorer(cfg *config.Config, store store.TransactionStore, cache *cache.VelocityCache) *RiskScorer {
	return &RiskScorer{cfg: cfg, store: store, cache: cache}
}

// Score evaluates a transaction request and returns the risk assessment.
func (rs *RiskScorer) Score(ctx context.Context, req models.ScoreRequest) (*models.RiskScoreResponse, *models.Transaction, error) {
	cardHash := utils.HashCard(req.CardNumber)
	now := time.Now()
	if req.Timestamp != "" {
		if t, err := time.Parse(time.RFC3339, req.Timestamp); err == nil {
			now = t
		}
	}

	flags := []string{}
	score := 0

	// 1. Amount Anomaly
	avgAmt, _ := rs.store.AvgAmountByCustomer(ctx, req.CustomerID)
	if req.Amount > rs.cfg.AmountGlobalHighValue {
		flags = append(flags, "high_value_transaction")
		score += rs.cfg.WeightAmountAnomaly
	} else if avgAmt > 0 && req.Amount > avgAmt*rs.cfg.AmountAnomalyMultiplier {
		flags = append(flags, "amount_anomaly_above_customer_average")
		score += rs.cfg.WeightAmountAnomaly
	}

	// 2. Velocity Card
	cardCount := rs.getCount(ctx, "card_"+cardHash, func() (int64, error) {
		return rs.store.CountByCardHash(ctx, cardHash, now.Add(-rs.cfg.VelocityCardWindow))
	})
	if cardCount >= int64(rs.cfg.VelocityCardLimit) {
		flags = append(flags, "velocity_card_exceeded")
		score += rs.cfg.WeightVelocityCard
	}

	// 3. Velocity Email
	emailCount := rs.getCount(ctx, "email_"+req.CustomerEmail, func() (int64, error) {
		return rs.store.CountByEmail(ctx, req.CustomerEmail, now.Add(-rs.cfg.VelocityEmailWindow))
	})
	if emailCount >= int64(rs.cfg.VelocityEmailLimit) {
		flags = append(flags, "velocity_email_exceeded")
		score += rs.cfg.WeightVelocityEmail
	}

	// 4. Velocity Shipping (distinct cards)
	shipCount := rs.getCount(ctx, "ship_"+req.ShippingAddress, func() (int64, error) {
		return rs.store.CountDistinctCardsByShipping(ctx, req.ShippingAddress, now.Add(-rs.cfg.VelocityShippingWindow))
	})
	if shipCount >= int64(rs.cfg.VelocityShippingLimit) {
		flags = append(flags, "velocity_shipping_exceeded")
		score += rs.cfg.WeightVelocityShipping
	}

	// 5. Velocity IP
	ipCount := rs.getCount(ctx, "ip_"+req.IPAddress, func() (int64, error) {
		return rs.store.CountByIP(ctx, req.IPAddress, now.Add(-rs.cfg.VelocityIPWindow))
	})
	if ipCount >= int64(rs.cfg.VelocityIPLimit) {
		flags = append(flags, "velocity_ip_exceeded")
		score += rs.cfg.WeightVelocityIP
	}

	// 6. Geographic Mismatch
	if strings.ToLower(req.BillingCountry) != strings.ToLower(req.ShippingCountry) {
		flags = append(flags, "country_mismatch")
		score += rs.cfg.WeightGeoMismatch
	}

	// 7. First-Time High Value
	if req.IsFirstTime && req.Amount > rs.cfg.AmountFirstTimeHighValue {
		flags = append(flags, "high_value_first_purchase")
		score += rs.cfg.WeightFirstTimeHighValue
	}

	// 8. BIN Risk
	for _, bin := range rs.cfg.PrepaidBINs {
		if strings.HasPrefix(req.CardBIN, bin) {
			flags = append(flags, "prepaid_card_bin")
			score += rs.cfg.WeightBINRisk
			break
		}
	}

	// 9. IP Reputation / Known Proxy-VPN
	for _, proxyIP := range rs.cfg.ProxyIPs {
		if req.IPAddress == proxyIP {
			flags = append(flags, "known_proxy_vpn_ip")
			score += rs.cfg.WeightIPReputation
			break
		}
	}
	// Also check high-risk history from this IP
	hrCount := rs.getCount(ctx, "hr_ip_"+req.IPAddress, func() (int64, error) {
		return rs.store.CountHighRiskByIP(ctx, req.IPAddress, now.Add(-rs.cfg.VelocityIPReputationWindow))
	})
	if hrCount >= int64(rs.cfg.VelocityIPReputationLimit) {
		flags = append(flags, "ip_reputation_bad")
		score += rs.cfg.WeightIPReputation
	}

	score = int(math.Min(100, math.Max(0, float64(score))))

	level, rec := rs.classify(score)

	flagsJSON, _ := json.Marshal(flags)

	tx := &models.Transaction{
		TransactionID:   req.TransactionID,
		Timestamp:       now,
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
		Score:           score,
		RiskLevel:       level,
		Flags:           string(flagsJSON),
		Recommendation:  rec,
	}

	resp := &models.RiskScoreResponse{
		TransactionID:  req.TransactionID,
		Score:          score,
		RiskLevel:      level,
		Flags:          flags,
		Recommendation: rec,
	}
	return resp, tx, nil
}

func (rs *RiskScorer) getCount(ctx context.Context, cacheKey string, query func() (int64, error)) int64 {
	// For prototype correctness, query DB directly to avoid stale cache on async writes.
	// In production, implement cache invalidation on writes.
	count, err := query()
	if err != nil {
		return 0
	}
	return count
}

func (rs *RiskScorer) classify(score int) (level, recommendation string) {
	switch {
	case score <= rs.cfg.ThresholdMedium:
		return "LOW", "approve"
	case score <= rs.cfg.ThresholdHigh:
		return "MEDIUM", "review"
	case score <= 90:
		return "HIGH", "decline"
	default:
		return "CRITICAL", "decline"
	}
}
