.PHONY: fmt vet test build integration migration-roundtrip db-up migrate-up migrate-down migrate-version verify

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

vet:
	go vet ./...

test:
	go test -race ./...

build:
	go build ./cmd/api

integration:
	go test -tags=integration ./test/integration

migration-roundtrip:
	go run ./cmd/migrate up
	go run ./cmd/migrate down 1
	go run ./cmd/migrate up

db-up:
	docker compose up -d --wait db

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down 1

migrate-version:
	go run ./cmd/migrate version

verify: fmt vet test migration-roundtrip integration build
