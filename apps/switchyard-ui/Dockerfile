# Production Dockerfile for Enclii Switchyard UI (Next.js)
FROM node:20-alpine AS base
RUN apk add --no-cache libc6-compat
WORKDIR /app

# Install dependencies
FROM base AS deps
COPY package.json package-lock.json ./
RUN npm install --legacy-peer-deps

# Build the application
FROM base AS builder
COPY --from=deps /app/node_modules ./node_modules
COPY . .

# Build arguments for public env vars
ARG NEXT_PUBLIC_API_URL=http://localhost:4200
ARG NEXT_PUBLIC_AUTH_MODE=local
ARG NEXT_PUBLIC_JANUA_URL=http://localhost:8000
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_AUTH_MODE=$NEXT_PUBLIC_AUTH_MODE
ENV NEXT_PUBLIC_JANUA_URL=$NEXT_PUBLIC_JANUA_URL
ENV NEXT_TELEMETRY_DISABLED=1

RUN npm run build

# Production image
FROM base AS runner
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

RUN addgroup --system --gid 1001 nodejs
RUN adduser --system --uid 1001 nextjs

WORKDIR /app

# Copy standalone build
COPY --from=builder /app/public ./public
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static

USER nextjs

EXPOSE 8030

ENV PORT=8030
ENV HOSTNAME="0.0.0.0"

CMD ["node", "server.js"]
