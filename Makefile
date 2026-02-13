default: test

SHELL=/bin/bash

SD_DB?="postgresql://pg:pass@localhost:5432/status_dashboard?sslmode=disable"
GOLANGCI_LINT_VERSION?="2.8.0"

test:
	@echo running unit tests
	go test ./internal/... -count 1

test-acc:
	@echo running integrational tests with docker and db
	go test ./tests/... -count 1

build:
	@echo build app
	go build -o app cmd/main.go

lint:
	@echo check linter version
	if [[ $$(golangci-lint --version |awk '{print $$4}') == $(GOLANGCI_LINT_VERSION) ]]; then echo "current installed version is actual to $(GOLANGCI_LINT_VERSION)"; else echo "current version is not actual, please use $(GOLANGCI_LINT_VERSION)"; exit 1; fi
	@echo running linter
	golangci-lint run -v

lint-check-version:
	@echo check linter version
	@if [[ $(GO_LINT) == v$(GOLANGCI_LINT_VERSION) ]]; then echo "current installed version is actual to $(GOLANGCI_LINT_VERSION)"; else echo "current version $(GO_LINT) is not actual, please use $(GOLANGCI_LINT_VERSION)"; exit 1; fi

migrate-up:
	@echo starting migrations
	@migrate -database $(SD_DB) -path db/migrations up 1

migrate-down:
	@echo revert migrations
	migrate -database $(SD_DB) -path db/migrations down 1

migrate-create:
	@echo create migration
	@migrate create -ext sql -dir db/migrations -seq $(name)

migrate-force:
	@echo force migration
	@migrate -database $(SD_DB) -path db/migrations force $(version)
