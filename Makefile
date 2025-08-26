.PHONY: bootstrap build-all build-api build-cli build-ui build-reconcilers
.PHONY: test lint run-switchyard run-ui run-reconcilers
.PHONY: kind-up kind-down infra-dev clean

# Variables
REGISTRY ?= ghcr.io/madfam
VERSION ?= $(shell git describe --always --dirty)
KIND_CLUSTER_NAME ?= enclii

# Bootstrap development environment
bootstrap:
	@echo "🚂 Bootstrapping Enclii development environment..."
	go mod download
	cd apps/switchyard-ui && npm install
	@echo "✅ Bootstrap complete"

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
	@echo "🧪 Running tests..."
	go test ./...
	cd apps/switchyard-ui && npm test

test-integration:
	@echo "🧪 Running integration tests..."
	go test ./... -tags=integration

test-coverage:
	@echo "📊 Running tests with coverage..."
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

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

# Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -rf bin/
	rm -rf apps/switchyard-ui/dist
	rm -rf apps/switchyard-ui/.next