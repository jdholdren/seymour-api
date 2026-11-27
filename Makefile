.PHONY: start yac up up-d build rb-api lint install-hooks

test:
	go test ./...

lint:
	golangci-lint run ./...

install-hooks:
	git config core.hooksPath .githooks

up:
	docker compose up

up-d:
	docker compose up -d

build:
	docker compose build

rb-api:
	docker compose build api && \
	docker compose up -d --force-recreate api
