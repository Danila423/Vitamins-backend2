.PHONY: seed-catalog test-cache test-db-up test-db-create test-db-ready test-unit test-integration test-e2e test-all test-mocks testifylint test-tools

DOCKER_GO_IMAGE ?= golang:1.23-bookworm
DOCKER_TEST_NETWORK ?= vitamins-backend_2_full_default
DOCKER_TEST_DB_NAME ?= vitamins_test
DOCKER_TEST_DATABASE_URL ?= postgres://vitamins:vitamins@db:5432/$(DOCKER_TEST_DB_NAME)?sslmode=disable
GOTESTSUM_FORMAT ?= pkgname
GOTESTSUM_VERSION ?= v1.12.0
GO_TEST_P ?= 1

seed-catalog:
	@test -n "$(DATABASE_URL)" || (echo "DATABASE_URL is required" >&2; exit 1)
	psql "$(DATABASE_URL)" -f internal/db/seed_vitamin_catalog.sql

test-cache:
	@mkdir -p .cache/go-mod .cache/go-build

test-db-up:
	docker compose up -d db

test-db-create: test-db-up
	@docker compose exec -T db psql -U vitamins -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname = '$(DOCKER_TEST_DB_NAME)'" | grep -q 1 || \
		docker compose exec -T db psql -U vitamins -d postgres -c "CREATE DATABASE $(DOCKER_TEST_DB_NAME) OWNER vitamins;"

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
	docker run --rm \
		--network $(DOCKER_TEST_NETWORK) \
		-v "$(PWD)":/app \
		-v "$(PWD)/.cache/go-mod":/go/pkg/mod \
		-v "$(PWD)/.cache/go-build":/root/.cache/go-build \
		-w /app \
		-e TEST_DATABASE_URL="$(DOCKER_TEST_DATABASE_URL)" \
		$(DOCKER_GO_IMAGE) \
		/bin/sh -lc 'export PATH=/usr/local/go/bin:$$PATH; /usr/local/go/bin/go install gotest.tools/gotestsum@$(GOTESTSUM_VERSION); /go/bin/gotestsum --format $(GOTESTSUM_FORMAT) -- -tags=e2e -count=1 -p $(GO_TEST_P) ./test/e2e'

test-all: test-unit test-integration test-e2e testifylint

test-mocks:
	$$(go env GOPATH)/bin/mockery --name ServiceAPI --dir internal/auth --output internal/auth/mocks --outpkg mocks --filename service_api.go

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
	go install gotest.tools/gotestsum@$(GOTESTSUM_VERSION)
