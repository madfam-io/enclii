# Build stage
FROM ghcr.io/madfam-org/base/node:20-alpine AS builder

WORKDIR /app

# Install dependencies
# Support both repo root context (apps/landing/) and local context (.)
COPY apps/landing/package*.json ./
RUN npm ci

# Copy source files
COPY apps/landing/ .

# Build static site
RUN npm run build

# Production stage
FROM ghcr.io/madfam-org/base/nginx:alpine

# Copy nginx config
COPY apps/landing/nginx.conf /etc/nginx/conf.d/default.conf

# Copy built static files
COPY --from=builder /app/out /usr/share/nginx/html

# Expose port 80 for Kubernetes health checks (service handles external routing)
EXPOSE 80

# Start nginx
CMD ["nginx", "-g", "daemon off;"]
