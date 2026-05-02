.PHONY: all deps generate seed swag run test clean

all: deps generate seed swag run

deps:
	go mod tidy

generate:
	go run scripts/generate_test_data.go

seed:
	go run cmd/seed/main.go

swag:
	which swag || go install github.com/swaggo/swag/cmd/swag@latest
	swag init -g cmd/api/main.go -o api/swagger

run:
	go run cmd/api/main.go

test:
	GOMAXPROCS=1 go test -v ./internal/...

curl-test:
	@echo "=== Low Risk Example ==="
	@curl -s -X POST http://localhost:8080/api/v1/score \
		-H "Content-Type: application/json" \
		-d @examples/low_risk.json
	@echo ""
	@echo "=== High Risk Example ==="
	@curl -s -X POST http://localhost:8080/api/v1/score \
		-H "Content-Type: application/json" \
		-d @examples/high_risk.json

clean:
	rm -f fraud_scorer.db testdata/transactions.json
