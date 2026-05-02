# LuxeCart Real-Time Fraud Scorer

A real-time transaction risk scoring API built for LuxeCart, a Dubai-based luxury marketplace. It evaluates transactions during payment authorization using a rule-based weighted engine, detecting velocity attacks, geographic mismatches, high-value anomalies, BIN risks, and known proxy/VPN usage.

## Tech Stack

- **Go 1.22+** with **Gin** HTTP framework
- **GORM** + **SQLite** (`github.com/glebarez/sqlite`, pure-Go, no CGO)
- **Swagger/OpenAPI** auto-generated docs via `swaggo`
- **Goroutines + channels** for concurrency-safe async persistence
- **`sync.WaitGroup`** for graceful worker pool shutdown

## Project Structure

Standard Go project layout:

```
.
├── cmd/
│   ├── api/main.go              # HTTP server entry point
│   └── seed/main.go             # Seed test data into SQLite
├── internal/
│   ├── config/                  # Environment-based configuration
│   ├── models/                  # GORM models + request/response DTOs
│   ├── store/                   # GORM repository (query by card, email, IP, shipping)
│   ├── scorer/                  # Rule-based scoring engine (stateless, synchronous)
│   ├── cache/                   # sync.RWMutex velocity cache + background eviction
│   ├── handler/                 # Gin HTTP handlers + integration tests
│   ├── worker/                  # Buffered channel + worker goroutines for async writes
│   └── seed/                    # Test data seeding logic
├── pkg/utils/                   # Card hashing helpers
├── scripts/
│   └── generate_test_data.go    # Generates ~300 synthetic transactions
├── testdata/
│   └── transactions.json        # Generated dataset
├── examples/
│   ├── low_risk.json            # Example low-risk request
│   └── high_risk.json           # Example high-risk request
├── api/swagger/                 # Auto-generated OpenAPI docs (compiled into binary)
├── Makefile
├── go.mod
├── go.sum
├── README.md
└── ARCHITECTURE.md
```

## Quick Start

### 1. Install Dependencies

```bash
make deps
```

### 2. Generate Test Data

```bash
make generate
```

Creates `testdata/transactions.json` with ~300 transactions including:
- Normal legitimate transactions (70-80%)
- Velocity attack (8 transactions, same card, 8 minutes)
- Shipping address scam (6 orders to same address, different cards)
- High-value first-time purchases (>$20k)
- Geographic mismatches (billing != shipping)
- Known proxy/VPN IPs
- Repeat loyal customers with consistent patterns
- Currencies: USD, EUR, AED

### 3. Seed the Database

```bash
make seed
```

Loads all test transactions into SQLite so the scoring engine has historical context.

### 4. Generate Swagger Docs

```bash
make swag
```

### 5. Run the Server

```bash
make run
```

The API will be available at `http://localhost:8080`.

- **Swagger UI**: `http://localhost:8080/swagger/index.html`
- **Health check**: `GET /health`

## API Endpoints

### Score a Transaction

`POST /api/v1/score`

Returns a real-time risk assessment during authorization.

**Example Request:**

```bash
curl -X POST http://localhost:8080/api/v1/score \
  -H "Content-Type: application/json" \
  -d @examples/high_risk.json
```

**Example Response:**

```json
{
  "transaction_id": "TX-DEMO-HIGH-001",
  "score": 80,
  "risk_level": "HIGH",
  "flags": [
    "high_value_transaction",
    "country_mismatch",
    "high_value_first_purchase",
    "prepaid_card_bin",
    "known_proxy_vpn_ip"
  ],
  "recommendation": "decline"
}
```

### Ingest Historical Transaction

`POST /api/v1/transactions`

Store a transaction for historical pattern analysis.

### List Transactions

`GET /api/v1/transactions?limit=20&offset=0`

### Submit Feedback

`POST /api/v1/feedback`

Mark a transaction as confirmed fraud or legitimate.

```bash
curl -X POST http://localhost:8080/api/v1/feedback \
  -H "Content-Type: application/json" \
  -d '{"transaction_id": "TX-123", "confirmed_status": "fraud"}'
```

## Testing

### Run Unit & Integration Tests

```bash
make test
```

Runs 19 tests across all packages:
- `internal/cache` — cache get/set, TTL expiration, concurrent access
- `internal/store` — GORM create, count, average, feedback updates
- `internal/scorer` — rule engine: low-risk, high-value, velocity, BIN, geo mismatch, clamping
- `internal/handler` — HTTP handler unit tests + 7 end-to-end integration scenarios

### Live API Testing with curl

```bash
# Low-risk loyal customer (should score LOW -> approve)
curl -s -X POST http://localhost:8080/api/v1/score \
  -H "Content-Type: application/json" \
  -d @examples/low_risk.json

# High-risk attack profile (should score HIGH -> decline)
curl -s -X POST http://localhost:8080/api/v1/score \
  -H "Content-Type: application/json" \
  -d @examples/high_risk.json
```

Or use `make curl-test` (requires a running server).

## Configurable Rules

All thresholds and weights are configurable via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `SCORE_THRESHOLD_MEDIUM` | 30 | Upper bound for LOW risk |
| `SCORE_THRESHOLD_HIGH` | 70 | Upper bound for MEDIUM risk |
| `VELOCITY_CARD_LIMIT` | 3 | Max card uses in window |
| `VELOCITY_CARD_WINDOW` | 15m | Card velocity window |
| `VELOCITY_IP_LIMIT` | 10 | Max transactions per IP in window |
| `VELOCITY_IP_WINDOW` | 10m | IP velocity window |
| `WEIGHT_AMOUNT_ANOMALY` | 20 | Points for amount anomaly |
| `WEIGHT_VELOCITY_CARD` | 25 | Points for card velocity |
| `WEIGHT_GEO_MISMATCH` | 10 | Points for country mismatch |
| `WEIGHT_IP_REPUTATION` | 20 | Points for bad IP reputation |

## Makefile Commands

| Command | Description |
|---------|-------------|
| `make deps` | Install Go dependencies |
| `make generate` | Generate synthetic test data |
| `make seed` | Load test data into SQLite |
| `make swag` | Generate OpenAPI docs |
| `make run` | Start the API server |
| `make test` | Run unit & integration tests |
| `make curl-test` | Run example curl requests against running server |
| `make clean` | Remove DB, test data, and binaries |
