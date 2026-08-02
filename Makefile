.PHONY: start yac up up-d build rb-api rb-worker gen-openapi

test:
	go test ./...

gen-openapi:
	go run ./cmd/genopenapi -out openapi.yaml

up:
	docker compose up

up-d:
	docker compose up -d

build:
	docker compose build

rb-api:
	docker compose build api && \
	docker compose up -d --force-recreate api

rb-worker:
	docker compose build worker && \
	docker compose up -d --force-recreate worker
