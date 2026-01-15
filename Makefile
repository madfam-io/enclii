.PHONY: bootstrap install-hooks build-all build-api build-cli build-ui build-reconcilers
.PHONY: test test-integration test-coverage test-benchmark test-all lint 
.PHONY: run-switchyard run-ui run-reconcilers
.PHONY: kind-up kind-down infra-dev deploy-staging deploy-prod health-check clean

# Variables
REGISTRY ?= ghcr.io/madfam
VERSION ?= $(shell git describe --always --dirty)
KIND_CLUSTER_NAME ?= enclii

# Bootstrap development environment
bootstrap:
	@echo "🚂 Bootstrapping Enclii development environment..."
	go mod download
	cd apps/switchyard-ui && npm install
	@echo "🔐 Installing git hooks..."
	@cp scripts/hooks/pre-commit .git/hooks/pre-commit 2>/dev/null || true
	@chmod +x .git/hooks/pre-commit 2>/dev/null || true
	@echo "✅ Bootstrap complete"

# Install pre-commit hooks (requires pre-commit to be installed)
install-hooks:
	@echo "🔐 Installing pre-commit hooks..."
	@if command -v pre-commit &> /dev/null; then \
		pre-commit install; \
	else \
		echo "⚠️  pre-commit not found, using built-in git hook"; \
		cp scripts/hooks/pre-commit .git/hooks/pre-commit; \
		chmod +x .git/hooks/pre-commit; \
	fi
	@echo "✅ Hooks installed"

# Build all components
build-all: build-api build-cli build-ui build-reconcilers

build-api:
	@echo "🏗️ Building Switchyard API..."
	cd apps/switchyard-api && go build -o ../../bin/switchyard-api ./cmd/api

build-cli:
	@echo "🏗️ Building CLI..."
	cd packages/cli && go build -o ../../bin/enclii ./cmd/enclii

build-ui:
	@echo "🏗️ Building UI..."
	cd apps/switchyard-ui && npm run build

build-reconcilers:
	@echo "🏗️ Building Reconcilers..."
	cd apps/reconcilers && go build -o ../../bin/reconcilers ./cmd/reconcilers

# Testing
test:
	@echo "🧪 Running unit tests..."
	cd apps/switchyard-api && go test -v -race -cover ./...
	cd packages/cli && go test -v -race -cover ./...
	cd apps/switchyard-ui && npm test

test-integration:
	@echo "🧪 Running integration tests..."
	cd apps/switchyard-api && go test -v -tags=integration ./...

test-coverage:
	@echo "📊 Generating test coverage report..."
	cd apps/switchyard-api && go test -coverprofile=coverage.out ./...
	cd apps/switchyard-api && go tool cover -html=coverage.out -o coverage.html
	cd packages/cli && go test -coverprofile=cli-coverage.out ./...
	cd packages/cli && go tool cover -html=cli-coverage.out -o cli-coverage.html
	@echo "Coverage reports generated"

test-benchmark:
	@echo "⚡ Running benchmark tests..."
	cd apps/switchyard-api && go test -bench=. -benchmem ./...
	cd packages/cli && go test -bench=. -benchmem ./...

test-all: test test-integration test-coverage
	@echo "✅ All tests completed successfully"

# Linting
lint:
	@echo "🔍 Linting code..."
	golangci-lint run ./...
	cd apps/switchyard-ui && npm run lint

# Run services locally
run-switchyard: build-api
	@echo "🚂 Starting Switchyard API on :8080..."
	./bin/switchyard-api

run-ui: build-ui
	@echo "🌐 Starting UI on :3000..."
	cd apps/switchyard-ui && npm run dev

run-reconcilers: build-reconcilers
	@echo "🔄 Starting Reconcilers..."
	./bin/reconcilers

# Kind cluster management
kind-up:
	@echo "🏗️ Creating kind cluster $(KIND_CLUSTER_NAME)..."
	kind create cluster --name $(KIND_CLUSTER_NAME) --config infra/dev/kind-config.yaml
	kubectl config use-context kind-$(KIND_CLUSTER_NAME)

kind-down:
	@echo "🗑️ Deleting kind cluster $(KIND_CLUSTER_NAME)..."
	kind delete cluster --name $(KIND_CLUSTER_NAME)

# Install development infrastructure
infra-dev:
	@echo "🏗️ Installing development infrastructure..."
	kubectl apply -f infra/dev/namespace.yaml
	kubectl apply -k infra/k8s/base
	@echo "⏳ Waiting for services to be ready..."
	kubectl wait --for=condition=ready pod -l app=postgres --timeout=300s
	kubectl wait --for=condition=ready pod -l app=redis --timeout=300s
	kubectl wait --for=condition=ready pod -l app=switchyard-api --timeout=300s

# Deploy to staging
deploy-staging:
	@echo "🚀 Deploying to staging environment..."
	kubectl create namespace enclii-staging --dry-run=client -o yaml | kubectl apply -f -
	kubectl apply -k infra/k8s/staging
	kubectl rollout status deployment/switchyard-api -n enclii-staging --timeout=300s

# Deploy to production  
deploy-prod:
	@echo "🚀 Deploying to production environment..."
	@echo "⚠️  Production deployment requires manual confirmation"
	@read -p "Deploy to production? (yes/no): " confirm && [ "$$confirm" = "yes" ]
	kubectl create namespace enclii-production --dry-run=client -o yaml | kubectl apply -f -
	kubectl apply -k infra/k8s/production
	kubectl rollout status deployment/switchyard-api -n enclii-production --timeout=600s

# Health check all environments
health-check:
	@echo "🏥 Checking health of all environments..."
	@echo "Development:"
	kubectl get pods -l app=switchyard-api || true
	@echo "Staging:"  
	kubectl get pods -l app=switchyard-api -n enclii-staging || true
	@echo "Production:"
	kubectl get pods -l app=switchyard-api -n enclii-production || true

# Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -rf bin/
	rm -rf apps/switchyard-ui/dist
	rm -rf apps/switchyard-ui/.next