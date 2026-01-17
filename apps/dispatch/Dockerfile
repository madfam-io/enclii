# Production Dockerfile for Enclii Dispatch (Infrastructure Control Tower)
# Build context should be the enclii root directory, not apps/dispatch
# Usage: docker build -f apps/dispatch/Dockerfile -t dispatch .

FROM ghcr.io/madfam-org/base/node:20-alpine AS base
RUN apk add --no-cache libc6-compat
RUN corepack enable pnpm
WORKDIR /app

# Install dependencies
FROM base AS deps
WORKDIR /app/dispatch
COPY apps/dispatch/package.json ./
# Use pnpm fetch for better caching
RUN pnpm install --frozen-lockfile=false

# Build the application
FROM base AS builder
WORKDIR /app

# Copy dispatch app
COPY apps/dispatch ./dispatch

# Install dependencies from deps stage
COPY --from=deps /app/dispatch/node_modules ./dispatch/node_modules

# Build arguments for public env vars (production defaults)
ARG NEXT_PUBLIC_APP_URL=https://admin.enclii.dev
ARG NEXT_PUBLIC_API_URL=https://api.enclii.dev
ARG NEXT_PUBLIC_JANUA_URL=https://auth.madfam.io
ENV NEXT_PUBLIC_APP_URL=$NEXT_PUBLIC_APP_URL
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_JANUA_URL=$NEXT_PUBLIC_JANUA_URL
ENV NEXT_TELEMETRY_DISABLED=1

# Build the static site
WORKDIR /app/dispatch
RUN pnpm run build

# Production image
FROM base AS runner
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

RUN addgroup --system --gid 1001 nodejs
RUN adduser --system --uid 1001 nextjs

WORKDIR /app

# Copy standalone build
COPY --from=builder /app/dispatch/public ./public
COPY --from=builder --chown=nextjs:nodejs /app/dispatch/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/dispatch/.next/static ./.next/static

USER nextjs

EXPOSE 4203

ENV PORT=4203
ENV HOSTNAME="0.0.0.0"

CMD ["node", "server.js"]
