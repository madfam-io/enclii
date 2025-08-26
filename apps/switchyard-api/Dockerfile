# Build stage
FROM golang:1.22-alpine AS builder

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
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/switchyard-api .

EXPOSE 8080

CMD ["./switchyard-api"]