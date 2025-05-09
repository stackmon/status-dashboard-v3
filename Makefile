default: test

SHELL=/bin/bash

SD_DB?="postgresql://pg:pass@localhost:5432/status_dashboard?sslmode=disable"
GOLANGCI_LINT_VERSION?="2.1.5"

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
	if [[ $(GO_LINT) == $(addprefix "v",$(GOLANGCI_LINT_VERSION)) ]]; then echo "current installed version is actual to $(GOLANGCI_LINT_VERSION)"; else echo "current version is not actual, please use $(GOLANGCI_LINT_VERSION)"; exit 1; fi

migrate-up:
	@echo staring migrations
	migrate -database $(SD_DB) -path db/migrations up

migrate-down:
	@echo revert migrations
	migrate -database $(SD_DB) -path db/migrations down
