default: test

SHELL=/bin/bash

SD_DB?="postgresql://pg:pass@localhost:5432/status_dashboard?sslmode=disable"

test:
	@echo running tests
	go test ./... -count 1

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
