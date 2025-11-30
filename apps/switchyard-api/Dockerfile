# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git for go mod download
RUN apk add --no-cache git

# Copy go.work and all module files first
COPY go.work ./
COPY apps/switchyard-api/go.mod apps/switchyard-api/go.sum ./apps/switchyard-api/
COPY packages/sdk-go/go.mod ./packages/sdk-go/

# Create stub directories for other modules referenced in go.work
RUN mkdir -p apps/reconcilers packages/cli && \
    echo 'module github.com/madfam/enclii/apps/reconcilers' > apps/reconcilers/go.mod && \
    echo 'go 1.24' >> apps/reconcilers/go.mod && \
    echo 'module github.com/madfam/enclii/packages/cli' > packages/cli/go.mod && \
    echo 'go 1.24' >> packages/cli/go.mod

# Download dependencies
WORKDIR /app/apps/switchyard-api
RUN go mod download

# Copy source code
WORKDIR /app
COPY apps/switchyard-api/ ./apps/switchyard-api/
COPY packages/sdk-go/ ./packages/sdk-go/

# Build the application
WORKDIR /app/apps/switchyard-api
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/switchyard-api ./cmd/api

# Final stage
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata wget
WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/switchyard-api .

EXPOSE 8001

HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
  CMD wget -q --spider http://localhost:8001/health || exit 1

CMD ["./switchyard-api"]
