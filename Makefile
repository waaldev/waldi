-include .env
export

.PHONY: build editor test run fmt dev lint migrate db

build: editor
	go build ./cmd/waldi

editor:
	cd web/editor && npm run build

test:
	go test ./...

run:
	go run ./cmd/waldi serve

fmt:
	gofmt -w cmd internal

dev:
	air & \
	(cd web/editor && npm run watch) & \
	wait

lint:
	golangci-lint run

migrate:
	go run ./cmd/waldi migrate

db:
	docker compose up -d postgres
