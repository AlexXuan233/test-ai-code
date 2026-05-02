# Architecture Overview

## Scoring Approach

The fraud scorer uses a **rule-based weighted additive model**. When a transaction arrives at `POST /api/v1/score`, the engine synchronously evaluates it against 9 rules: amount anomaly, velocity (card/email/IP/shipping), geographic mismatch, first-time high-value, BIN risk, and IP reputation. Each triggered rule contributes configurable points; the sum is clamped to 0-100. Thresholds map scores to risk levels: LOW (0-30, approve), MEDIUM (31-70, review), HIGH/CRITICAL (71+, decline).

## Why Synchronous Scoring?

Payment authorization is a blocking step in checkout. The merchant must receive approve/review/decline in real time before the processor holds funds. Therefore the scoring path is entirely synchronous, targeting <50ms latency. The transaction is then persisted asynchronously via a buffered channel and worker goroutine pool so latency is never impacted by DB writes.

## Data Storage

SQLite was chosen via GORM (`github.com/glebarez/sqlite`, pure-Go, no CGO) to eliminate external database setup while still supporting production-grade SQL queries for velocity windows and aggregations. GORM's connection pool is goroutine-safe, and SQLite natively serializes writes. The schema auto-migrates on startup.

## Concurrency & Safety

- **sync.RWMutex** guards an in-memory velocity cache for hot keys (card/email/IP counts), reducing DB load.
- A **background goroutine** with `time.Ticker` evicts stale cache entries.
- **Buffered channels** with worker goroutines handle async transaction persistence and bulk seeding, preventing goroutine leaks and backpressure.
- All background workers accept `context.Context` for graceful shutdown.

## Extensibility

The architecture supports future ML-based scoring by keeping the `RiskScorer` interface stateless. Feedback loop data (`confirmed_status`) is already stored per transaction, enabling future score re-calibration. All rule weights and thresholds are externally configurable via environment variables.
