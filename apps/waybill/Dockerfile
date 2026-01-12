# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files (support repo root context with apps/waybill/ path)
COPY apps/waybill/go.mod apps/waybill/go.sum ./
RUN go mod download

# Copy source code
COPY apps/waybill/ .

# Build API binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /waybill-api ./cmd/api

# Build Aggregator binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /waybill-aggregator ./cmd/aggregator

# API runtime stage
FROM alpine:3.19 AS api

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /waybill-api /usr/local/bin/waybill-api

EXPOSE 8082

ENTRYPOINT ["/usr/local/bin/waybill-api"]

# Aggregator runtime stage
FROM alpine:3.19 AS aggregator

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /waybill-aggregator /usr/local/bin/waybill-aggregator

ENTRYPOINT ["/usr/local/bin/waybill-aggregator"]
