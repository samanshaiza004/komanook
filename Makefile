.PHONY: setup generate assets build run migrate seed check test

setup:
	npm ci
	go mod download

generate:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
	go run github.com/a-h/templ/cmd/templ@v0.3.1100 generate

assets:
	npm run assets
	npm run licenses

build:
	CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o bin/komanook ./cmd/komanook

run:
	go run ./cmd/komanook serve

migrate:
	go run ./cmd/komanook migrate

seed:
	go run ./cmd/komanook seed

check:
	go vet ./...
	go run github.com/a-h/templ/cmd/templ@v0.3.1100 fmt -fail .
	npm run lint
	test -z "$$(gofmt -l cmd internal db content)"

test:
	go test -race ./...
