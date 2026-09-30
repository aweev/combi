# Africa OS — root Makefile
.PHONY: help proto lint test audit infra-up infra-down infra-logs build tidy

GO      := go
BUF     := buf
COMPOSE := docker compose -f docker-compose.dev.yml

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

proto: ## Generate all code from .proto files
	$(BUF) lint
	$(BUF) generate
	@echo "✓ Proto generation complete"

proto-breaking: ## Check for breaking changes in proto
	$(BUF) breaking --against '.git#branch=main'

lint: ## Run golangci-lint across all services
	@for svc in services/*/; do \
		echo "linting $$svc"; \
		cd $$svc && golangci-lint run ./... && cd ../..; \
	done

lint-fix: ## Run golangci-lint with --fix
	@for svc in services/*/; do \
		cd $$svc && golangci-lint run --fix ./... && cd ../..; \
	done

test: ## Run all tests
	@for svc in services/*/; do \
		echo "testing $$svc"; \
		cd $$svc && $(GO) test -race -cover ./... && cd ../..; \
	done

audit: ## Run govulncheck and cargo-audit on all services
	govulncheck ./...
	cargo audit
	@echo "✓ Security audit complete"

tidy: ## Tidy all go.mod files
	@for svc in services/*/; do \
		cd $$svc && $(GO) mod tidy && cd ../..; \
	done

build: ## Build all service binaries
	@for svc in services/*/; do \
		svc_name=$$(basename $$svc); \
		echo "building $$svc_name"; \
		cd $$svc && $(GO) build -trimpath -ldflags="-s -w" ./cmd/server && cd ../..; \
	done

infra-up: ## Start local dev infra (Postgres, Redis, Redpanda, OTel, Grafana)
	$(COMPOSE) up -d
	@echo "✓ Infra up"
	@echo "  Grafana:    http://localhost:3000"
	@echo "  Prometheus: http://localhost:9090"
	@echo "  Jaeger:     http://localhost:16686"
	@echo "  Redpanda:   localhost:9092"
	@echo "  Postgres:   localhost:5432  (combi/combi)"
	@echo "  Redis:      localhost:6379"

infra-down: ## Stop local dev infra
	$(COMPOSE) down

infra-logs: ## Tail infra logs
	$(COMPOSE) logs -f

infra-reset: ## DANGER: destroy and recreate all infra volumes
	$(COMPOSE) down -v
	$(COMPOSE) up -d

.DEFAULT_GOAL := help
