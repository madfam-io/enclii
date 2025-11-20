# Build stage
# SECURITY FIX: Pin to specific Go version to ensure reproducible builds
FROM golang:1.24.7-alpine3.20 AS builder

WORKDIR /app

# Copy go mod files
COPY go.work go.work.sum ./
COPY apps/switchyard-api/go.mod apps/switchyard-api/go.sum ./apps/switchyard-api/
COPY packages/sdk-go/go.mod ./packages/sdk-go/

# Download dependencies
RUN go mod download

# Copy source code
COPY apps/switchyard-api/ ./apps/switchyard-api/
COPY packages/sdk-go/ ./packages/sdk-go/

# Build the application
RUN cd apps/switchyard-api && CGO_ENABLED=0 GOOS=linux go build -o /app/switchyard-api ./cmd/api

# Final stage
# SECURITY FIX: Pin to specific Alpine version to ensure reproducible builds and prevent supply chain attacks
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/switchyard-api .

EXPOSE 8080

CMD ["./switchyard-api"]