# Roundhouse - Build Pipeline Service

Roundhouse is Enclii's build pipeline service that handles git-triggered builds, image creation, and deployment preparation.

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Git Webhook   │────▶│  Roundhouse API │────▶│   Redis Queue   │
│  (GitHub/etc)   │     │    (Go/Gin)     │     │                 │
└─────────────────┘     └─────────────────┘     └────────┬────────┘
                                                         │
                        ┌─────────────────┐              │
                        │    Switchyard   │◀─────────────┤
                        │   (callback)    │              │
                        └─────────────────┘              │
                                                         ▼
                        ┌─────────────────┐     ┌─────────────────┐
                        │ Container       │◀────│   Worker(s)     │
                        │ Registry (GHCR) │     │  (BuildKit)     │
                        └─────────────────┘     └─────────────────┘
```

## Components

### API Server (`cmd/api`)
- Receives webhooks from GitHub/GitLab/Bitbucket
- Provides admin API for job management
- Enqueues build jobs to Redis

### Worker (`cmd/worker`)
- Dequeues and processes build jobs
- Clones repos, builds images via BuildKit
- Generates SBOMs (Syft) and signs images (Cosign)
- Pushes to container registry
- Sends callbacks to Switchyard

## Quick Start

```bash
# Start with Docker Compose
docker-compose up -d

# Or run locally
export REDIS_URL=redis://localhost:6379/0
export REGISTRY=ghcr.io

# Start API
go run ./cmd/api

# Start Worker (separate terminal)
go run ./cmd/worker
```

## Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `API_PORT` | API server port | `8081` |
| `REDIS_URL` | Redis connection URL | required |
| `REGISTRY` | Container registry | `ghcr.io` |
| `REGISTRY_USER` | Registry username | - |
| `REGISTRY_PASSWORD` | Registry password/token | - |
| `BUILD_WORK_DIR` | Temp directory for builds | `/tmp/roundhouse-builds` |
| `BUILD_TIMEOUT` | Max build duration | `30m` |
| `MAX_CONCURRENT_BUILDS` | Worker concurrency | `3` |
| `GENERATE_SBOM` | Generate SBOM with Syft | `true` |
| `SIGN_IMAGES` | Sign images with Cosign | `true` |
| `COSIGN_KEY` | Cosign private key path | - |
| `GITHUB_WEBHOOK_SECRET` | GitHub webhook secret | - |
| `SWITCHYARD_INTERNAL_URL` | Switchyard callback URL | - |
| `SWITCHYARD_API_KEY` | API key for callbacks | - |

## API Endpoints

### Webhooks
```
POST /webhooks/github     # GitHub push/PR events
POST /webhooks/gitlab     # GitLab push/MR events
POST /webhooks/bitbucket  # Bitbucket events
```

### Internal (Switchyard)
```
POST /internal/enqueue    # Enqueue build job
```

### Admin API
```
GET  /api/v1/jobs              # List jobs
GET  /api/v1/jobs/:id          # Get job details
GET  /api/v1/jobs/:id/logs     # Stream logs (SSE)
POST /api/v1/jobs/:id/cancel   # Cancel job
POST /api/v1/jobs/:id/retry    # Retry job
GET  /api/v1/workers           # List workers
GET  /api/v1/stats             # Build stats
```

### Health
```
GET /health   # Health check
GET /ready    # Readiness check
```

## Enqueue Request

```json
{
  "release_id": "uuid",
  "service_id": "uuid",
  "project_id": "uuid",
  "git_repo": "https://github.com/org/repo.git",
  "git_sha": "abc123def456",
  "git_branch": "main",
  "build_config": {
    "type": "dockerfile",
    "dockerfile": "Dockerfile",
    "context": ".",
    "build_args": {
      "NODE_ENV": "production"
    },
    "target": "production"
  },
  "callback_url": "https://switchyard/internal/build-complete",
  "priority": 0
}
```

## Build Result (Callback)

```json
{
  "job_id": "uuid",
  "release_id": "uuid",
  "success": true,
  "image_uri": "ghcr.io/project/service:abc123",
  "image_digest": "sha256:...",
  "image_size_mb": 250.5,
  "sbom": "{...}",
  "sbom_format": "spdx-json",
  "image_signature": "...",
  "duration_secs": 45.2,
  "logs_url": "https://roundhouse/api/v1/jobs/uuid/logs"
}
```

## Build Types

### Dockerfile (default)
Builds using a Dockerfile in the repository.

### Buildpack
Uses Cloud Native Buildpacks for language detection and building.

### Auto
Attempts to detect the best build method based on repository contents.

## Scaling

Workers can be horizontally scaled. Each worker:
- Registers itself in Redis
- Processes up to `MAX_CONCURRENT_BUILDS` jobs
- Gracefully shuts down (waits for active builds)

```bash
# Scale workers
docker-compose up -d --scale worker=3
```

## Integration with Switchyard

Roundhouse integrates with Switchyard (Enclii's main API) via:

1. **Enqueue**: Switchyard calls `/internal/enqueue` to start builds
2. **Callback**: Roundhouse POSTs results to Switchyard's callback URL
3. **Database**: Both share the PostgreSQL database for consistency

## Security

- Webhook signatures validated (HMAC-SHA256)
- Internal API protected by API key
- Images signed with Cosign
- SBOM generated for supply chain security
- Builds run in isolated containers
