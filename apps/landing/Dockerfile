# Build stage
FROM node:20-alpine AS builder

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
FROM nginx:alpine

# Copy nginx config
COPY apps/landing/nginx.conf /etc/nginx/conf.d/default.conf

# Copy built static files
COPY --from=builder /app/out /usr/share/nginx/html

# Expose port (4204 per PORT_ALLOCATION.md - Enclii landing page)
EXPOSE 4204

# Start nginx
CMD ["nginx", "-g", "daemon off;"]
