package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// Server
	Port string

	// Score thresholds
	ThresholdMedium int
	ThresholdHigh   int

	// Velocity limits and windows
	VelocityCardLimit          int
	VelocityCardWindow         time.Duration
	VelocityEmailLimit         int
	VelocityEmailWindow        time.Duration
	VelocityShippingLimit      int
	VelocityShippingWindow     time.Duration
	VelocityIPLimit            int
	VelocityIPWindow           time.Duration
	VelocityIPReputationLimit  int
	VelocityIPReputationWindow time.Duration

	// Amount anomalies
	AmountFirstTimeHighValue float64
	AmountGlobalHighValue    float64
	AmountAnomalyMultiplier  float64

	// Rule weights
	WeightAmountAnomaly      int
	WeightVelocityCard       int
	WeightVelocityEmail      int
	WeightVelocityShipping   int
	WeightVelocityIP         int
	WeightGeoMismatch        int
	WeightFirstTimeHighValue int
	WeightBINRisk            int
	WeightIPReputation       int

	// BIN risk
	PrepaidBINs []string
	ProxyIPs    []string

	// Async worker
	AsyncWriteChanSize int
	AsyncWriteWorkers  int

	// Cache
	CacheTTL       time.Duration
	CacheEviction  time.Duration
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		Port: getEnv("PORT", "8080"),

		ThresholdMedium: getIntEnv("SCORE_THRESHOLD_MEDIUM", 30),
		ThresholdHigh:   getIntEnv("SCORE_THRESHOLD_HIGH", 70),

		VelocityCardLimit:          getIntEnv("VELOCITY_CARD_LIMIT", 3),
		VelocityCardWindow:         getDurationEnv("VELOCITY_CARD_WINDOW", 15*time.Minute),
		VelocityEmailLimit:         getIntEnv("VELOCITY_EMAIL_LIMIT", 5),
		VelocityEmailWindow:        getDurationEnv("VELOCITY_EMAIL_WINDOW", 30*time.Minute),
		VelocityShippingLimit:      getIntEnv("VELOCITY_SHIPPING_LIMIT", 4),
		VelocityShippingWindow:     getDurationEnv("VELOCITY_SHIPPING_WINDOW", time.Hour),
		VelocityIPLimit:            getIntEnv("VELOCITY_IP_LIMIT", 10),
		VelocityIPWindow:           getDurationEnv("VELOCITY_IP_WINDOW", 10*time.Minute),
		VelocityIPReputationLimit:  getIntEnv("VELOCITY_IP_REPUTATION_LIMIT", 3),
		VelocityIPReputationWindow: getDurationEnv("VELOCITY_IP_REPUTATION_WINDOW", time.Hour),

		AmountFirstTimeHighValue: getFloatEnv("AMOUNT_FIRST_TIME_HIGH", 5000),
		AmountGlobalHighValue:    getFloatEnv("AMOUNT_GLOBAL_HIGH", 15000),
		AmountAnomalyMultiplier:  getFloatEnv("AMOUNT_ANOMALY_MULTIPLIER", 2.0),

		WeightAmountAnomaly:      getIntEnv("WEIGHT_AMOUNT_ANOMALY", 20),
		WeightVelocityCard:       getIntEnv("WEIGHT_VELOCITY_CARD", 25),
		WeightVelocityEmail:      getIntEnv("WEIGHT_VELOCITY_EMAIL", 15),
		WeightVelocityShipping:   getIntEnv("WEIGHT_VELOCITY_SHIPPING", 20),
		WeightVelocityIP:         getIntEnv("WEIGHT_VELOCITY_IP", 20),
		WeightGeoMismatch:        getIntEnv("WEIGHT_GEO_MISMATCH", 10),
		WeightFirstTimeHighValue: getIntEnv("WEIGHT_FIRST_TIME_HIGH", 15),
		WeightBINRisk:            getIntEnv("WEIGHT_BIN_RISK", 15),
		WeightIPReputation:       getIntEnv("WEIGHT_IP_REPUTATION", 20),

		PrepaidBINs: []string{"5105", "4111", "4000"},
		ProxyIPs:    []string{"10.0.0.50", "10.0.0.51", "10.0.0.52"},

		AsyncWriteChanSize: getIntEnv("ASYNC_WRITE_CHAN_SIZE", 1000),
		AsyncWriteWorkers:  getIntEnv("ASYNC_WRITE_WORKERS", 2),

		CacheTTL:      getDurationEnv("CACHE_TTL", 5*time.Minute),
		CacheEviction: getDurationEnv("CACHE_EVICTION", 60*time.Second),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getIntEnv(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func getFloatEnv(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func getDurationEnv(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
