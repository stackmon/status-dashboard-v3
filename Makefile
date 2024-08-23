default: test

test:
	@echo running tests
	go test ./... -count 1

build:
	@echo build app
	go build -o app cmd/main.go

lint:
	@echo running linter
	golangci-lint run -v
