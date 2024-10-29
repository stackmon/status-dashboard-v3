default: test

SHELL=/bin/bash

SD_DB?="postgresql://pg:pass@localhost:5432/status_dashboard?sslmode=disable"

test:
	@echo running unit tests
	go test ./internal/... -count 1

integ_test:
	@echo running integrational tests with docker and db
	go test ./tests/... -count 1

build:
	@echo build app
	go build -o app cmd/main.go

lint:
	@echo running linter
	golangci-lint run -v

migrate-up:
	@echo staring migrations
	migrate -database $(SD_DB) -path db/migrations up

migrate-down:
	@echo revert migrations
	migrate -database $(SD_DB) -path db/migrations down
