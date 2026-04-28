.PHONY: seed-catalog test-cache test-db-up test-db-create test-db-ready test-unit test-integration test-e2e test-all test-mocks testifylint test-tools \
	proto build-all build-gateway build-auth build-vitamins build-analytics \
	docker-build docker-up docker-down

DOCKER_GO_IMAGE ?= golang:1.25-bookworm
COMPOSE_FILE ?= deploy/docker-compose.yml
DOCKER_COMPOSE ?= docker compose -f $(COMPOSE_FILE)
DOCKER_TEST_DB_CONTAINER ?= $(shell $(DOCKER_COMPOSE) ps -q db 2>/dev/null)
DOCKER_TEST_NETWORK ?= $(shell docker inspect $(DOCKER_TEST_DB_CONTAINER) --format '{{range $$k, $$v := .NetworkSettings.Networks}}{{$$k}}{{end}}' 2>/dev/null)
DOCKER_TEST_DB_NAME ?= vitamins_test
DOCKER_TEST_DATABASE_URL ?= postgres://vitamins:vitamins@db:5432/$(DOCKER_TEST_DB_NAME)?sslmode=disable
GOTESTSUM_FORMAT ?= pkgname
GOTESTSUM_VERSION ?= v1.13.0
GO_TEST_P ?= 1

seed-catalog:
	@test -n "$(DATABASE_URL)" || (echo "DATABASE_URL is required" >&2; exit 1)
	psql "$(DATABASE_URL)" -f pkg/db/seed_vitamin_catalog.sql

test-cache:
	@mkdir -p .cache/go-mod .cache/go-build

test-db-up:
	@$(DOCKER_COMPOSE) up -d db

test-db-create: test-db-up
	@$(DOCKER_COMPOSE) exec -T db psql -U vitamins -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname = '$(DOCKER_TEST_DB_NAME)'" | grep -q 1 || \
	$(DOCKER_COMPOSE) exec -T db psql -U vitamins -d postgres -c "CREATE DATABASE $(DOCKER_TEST_DB_NAME) OWNER vitamins;"

test-db-ready: test-db-create
	@echo "test database is ready: $(DOCKER_TEST_DB_NAME)"

test-unit: test-cache
	docker run --rm \
		-v "$(PWD)":/app \
		-v "$(PWD)/.cache/go-mod":/go/pkg/mod \
		-v "$(PWD)/.cache/go-build":/root/.cache/go-build \
		-w /app \
		$(DOCKER_GO_IMAGE) \
		/bin/sh -lc 'export PATH=/usr/local/go/bin:$$PATH; /usr/local/go/bin/go install gotest.tools/gotestsum@$(GOTESTSUM_VERSION); /go/bin/gotestsum --format $(GOTESTSUM_FORMAT) -- -count=1 ./...'

test-integration: test-db-ready test-cache
	@test -n "$(DOCKER_TEST_NETWORK)" || (echo "Docker test network not found. Run: make test-db-up" >&2; exit 1)
	docker run --rm \
		--network $(DOCKER_TEST_NETWORK) \
		-v "$(PWD)":/app \
		-v "$(PWD)/.cache/go-mod":/go/pkg/mod \
		-v "$(PWD)/.cache/go-build":/root/.cache/go-build \
		-w /app \
		-e TEST_DATABASE_URL="$(DOCKER_TEST_DATABASE_URL)" \
		$(DOCKER_GO_IMAGE) \
		/bin/sh -lc 'export PATH=/usr/local/go/bin:$$PATH; /usr/local/go/bin/go install gotest.tools/gotestsum@$(GOTESTSUM_VERSION); /go/bin/gotestsum --format $(GOTESTSUM_FORMAT) -- -tags=integration -count=1 -p $(GO_TEST_P) ./...'

test-e2e: test-db-ready test-cache
	@test -n "$(DOCKER_TEST_NETWORK)" || (echo "Docker test network not found. Run: make test-db-up" >&2; exit 1)
	docker run --rm \
		--network $(DOCKER_TEST_NETWORK) \
		-v "$(PWD)":/app \
		-v "$(PWD)/.cache/go-mod":/go/pkg/mod \
		-v "$(PWD)/.cache/go-build":/root/.cache/go-build \
		-w /app \
		-e TEST_DATABASE_URL="$(DOCKER_TEST_DATABASE_URL)" \
		-e GATEWAY_URL="http://gateway:8080" \
		$(DOCKER_GO_IMAGE) \
		/bin/sh -lc 'export PATH=/usr/local/go/bin:$$PATH; /usr/local/go/bin/go install gotest.tools/gotestsum@$(GOTESTSUM_VERSION); /go/bin/gotestsum --format $(GOTESTSUM_FORMAT) -- -tags=e2e -count=1 -p $(GO_TEST_P) ./test/e2e'

test-all: test-unit test-integration test-e2e testifylint

test-mocks:
	$$(go env GOPATH)/bin/mockery --name ServiceAPI --dir services/auth/internal/service --output services/auth/internal/mocks --outpkg mocks --filename service_api.go

testifylint: test-cache
	docker run --rm \
		-v "$(PWD)":/app \
		-v "$(PWD)/.cache/go-mod":/go/pkg/mod \
		-v "$(PWD)/.cache/go-build":/root/.cache/go-build \
		-w /app \
		$(DOCKER_GO_IMAGE) \
		/bin/sh -lc 'export PATH=/usr/local/go/bin:$$PATH; /usr/local/go/bin/go install github.com/Antonboom/testifylint@latest; /go/bin/testifylint ./...'

test-tools:
	go install github.com/vektra/mockery/v2@v2.53.3
	go install github.com/Antonboom/testifylint@v1.6.4
	go install gotest.tools/gotestsum@v1.13.0

proto:
	protoc --go_out=gen/go --go_opt=paths=source_relative \
		--go-grpc_out=gen/go --go-grpc_opt=paths=source_relative \
		-I proto \
		proto/auth/v1/auth.proto \
		proto/vitamins/v1/vitamins.proto \
		proto/analytics/v1/analytics.proto

build-gateway:
	CGO_ENABLED=0 go build -o bin/gateway ./services/gateway/cmd

build-auth:
	CGO_ENABLED=0 go build -o bin/auth-service ./services/auth/cmd

build-vitamins:
	CGO_ENABLED=0 go build -o bin/vitamins-service ./services/vitamins/cmd

build-analytics:
	CGO_ENABLED=0 go build -o bin/analytics-service ./services/analytics/cmd

build-all: build-gateway build-auth build-vitamins build-analytics

docker-build:
	$(DOCKER_COMPOSE) build

docker-up:
	$(DOCKER_COMPOSE) up -d

docker-down:
	$(DOCKER_COMPOSE) down
