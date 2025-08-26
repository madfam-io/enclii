# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Enclii is a Railway-style internal platform for building, deploying, scaling, and operating containerized services with guardrails. It runs on managed Kubernetes and managed databases, abstracting infrastructure complexity while maintaining operational safety.

## Architecture

The project follows a monorepo structure with these key components:

- **Switchyard**: Control plane API (Go) - manages projects, environments, services, deployments
- **Conductor (CLI)**: Developer interface (`enclii` command) (Go)
- **Roundhouse**: Build/provenance/signing workers (Go)
- **Reconcilers**: Kubernetes operators/controllers (Go)
- **UI**: Web interface (Next.js)
- **Junctions**: Routing/ingress + certs + DNS
- **Timetable**: Cron and one-off jobs
- **Lockbox**: Secrets management (Vault/1Password)
- **Signal**: Observability stack (logs/metrics/traces)
- **Waybill**: Cost tracking and budget alerts

## Common Development Commands

### Setup & Bootstrap
```bash
make bootstrap          # Install hooks, dependencies, and configure workspaces
make kind-up           # Create local kind cluster
make infra-dev         # Install ingress, cert-manager, observability stack
make dns-dev           # Configure dev DNS entries
```

### Running Services Locally
```bash
make run-switchyard    # Start control plane API on :8080
make run-ui            # Start web UI on :3000
make run-reconcilers   # Start Kubernetes controllers
```

### Building
```bash
make build-all         # Build all components
make build-cli         # Build CLI only
```

### Testing & Quality
```bash
make test              # Run unit tests
make e2e               # Run end-to-end tests
make lint              # Run linters (golangci-lint, eslint, prettier)
make precommit         # Run all checks before committing
```

### CLI Operations
```bash
./bin/enclii init                  # Scaffold a new service
./bin/enclii up                     # Deploy preview environment
./bin/enclii deploy --env prod     # Deploy to production
./bin/enclii logs <service> -f     # Tail service logs
./bin/enclii rollback <service>    # Rollback to previous release
```

## Key Technical Details

### Service Configuration
Services are defined using YAML specs (stored versioned in the control plane):
- Located at: Service spec embedded in control plane DB
- Format: `apiVersion: enclii.dev/v1`
- Includes: runtime config, health checks, routes, secrets, volumes, jobs, autoscaling

### Deployment Flow
1. Build via Nixpacks/Buildpacks or Dockerfile
2. Create immutable Release with provenance (git SHA, SBOM, signature)
3. Deploy with canary/blue-green strategies
4. Automatic rollback on failure based on SLO metrics

### Environment Variables
Key vars for local development (set in `.env`):
- `ENCLII_DB_URL`: Control plane database URL
- `ENCLII_REGISTRY`: Container registry
- `ENCLII_OIDC_ISSUER`: Auth provider URL
- `ENCLII_DEFAULT_REGION`: Default deployment region
- `ENCLII_LOG_LEVEL`: Logging verbosity

### Testing Strategy
- **Unit tests**: Test control plane validation, CLI arguments, reconciler idempotency
- **Integration tests**: Test build→release→deploy pipeline, secret injection, TLS issuance
- **E2E tests**: Test preview environment creation, canary deployments, rollbacks, cron jobs

### Platform SLOs
- Control plane API availability: 99.95% monthly
- Build subsystem availability: 99.9% monthly
- Preview environment provisioning: P95 < 3 minutes
- Deploy to stage: P95 ≤ 8 minutes for typical Node/Go services

## Important Conventions

### Security
- Never commit secrets - use Lockbox/Vault
- All images must be signed with cosign
- SBOM required for all releases
- Admission policies enforced via Kyverno/OPA

### Git Workflow
- Trunk-based development on `main`
- Conventional commits for changelog generation
- Preview environments created automatically for PRs
- Canary deploys to stage, then manual approval to prod

### Error Handling
- Exit codes: 0 (success), 10 (validation), 20 (build failed), 30 (deploy failed), 40 (timeout), 50 (auth)
- Precise error messages with actionable context
- Automatic rollback triggers when error rate > 2% for 2 minutes

### Cost Management
- Resource usage tracked per project/environment/service
- Budget alerts at 80% threshold
- Hard throttle at 100% for non-production environments