# Enclii

> **Control & orchestration for your cloud.**
> *Parallel rails for every deploy.*

**Status:** Alpha (private)
**Spec:** See [`SOFTWARE_SPEC.md`](./SOFTWARE_SPEC.md)
**Domain:** [https://enclii.dev](https://enclii.dev)

---

## What is Enclii?

Enclii is a Railway‑style internal platform that lets teams build, deploy, scale, and operate containerized services with guardrails. v1 runs on managed Kubernetes and managed databases, with a clear path to other substrates later.

---

## Repository layout (monorepo)

```
.
├── apps/
│   ├── switchyard-api/        # Control plane API (Go)
│   ├── switchyard-ui/         # Web UI (Next.js)
│   ├── roundhouse/            # Build/provenance/signing workers (Go)
│   └── reconcilers/           # K8s operators/controllers (Go)
├── packages/
│   ├── cli/                   # `enclii` CLI (Go)
│   ├── sdk-js/                # TypeScript SDK (Node 20)
│   └── sdk-go/                # Go SDK
├── infra/
│   ├── charts/                # Helm charts (Junctions, Signal, Timetable, etc.)
│   ├── terraform/             # Bootstrap IaC (cluster, DNS, buckets)
│   └── dev/                   # kind cluster, local overrides
├── docs/                      # Specs, ADRs, runbooks
├── .github/                   # CI/CD workflows
├── Taskfile.yml               # Task runner (optional)
├── Makefile                   # Common developer commands
└── go.work / package.json     # Go/Node workspace roots
```

> **Component names**
> **Switchyard** (control plane), **Conductor** (CLI), **Roundhouse** (builder), **Junctions** (ingress/DNS/TLS), **Timetable** (cron/jobs), **Lockbox** (secrets), **Signal** (obs), **Waybill** (costs).

---

## Architecture & Features

### Production-Ready Infrastructure

Enclii is built with production-grade reliability and observability:

**Security & Authentication:**
- JWT-based authentication with RSA signing
- Role-based access control (RBAC) with admin/developer/viewer roles
- Comprehensive input validation and sanitization
- Rate limiting and security middleware

**Performance & Scalability:**
- Redis caching with tag-based invalidation
- Database connection pooling with configurable limits
- Kubernetes-native horizontal pod autoscaling
- Efficient build caching and image layer reuse

**Observability & Monitoring:**
- Structured logging with OpenTelemetry tracing
- Prometheus metrics for all components
- Health checks for dependencies (database, cache, Kubernetes)
- Distributed tracing across request flows

**Operations & Reliability:**
- Automated backup and disaster recovery
- Rolling deployments with readiness/liveness probes
- Graceful shutdown handling
- Circuit breaker patterns for external dependencies

### Components

**Control Plane (`apps/switchyard-api`):**
- REST API with OpenAPI documentation
- PostgreSQL with migrations
- Background reconciliation controllers
- Kubernetes deployment orchestration

**Web UI (`apps/switchyard-ui`):**
- Next.js dashboard with Tailwind CSS
- Real-time deployment status updates
- Project and service management
- Log streaming and monitoring

**CLI (`packages/cli`):**
- Developer-friendly deployment workflow
- Service specification parsing
- Build and deployment automation
- Comprehensive error handling

---

## Prerequisites

* **Core:** Docker ≥ 24, Git, Make, Helm ≥ 3.14, kubectl ≥ 1.29
* **Languages:** Go ≥ 1.22, Node ≥ 20 (pnpm ≥ 9)
* **Local K8s:** kind ≥ 0.23 or k3d (we default to kind)
* **Security/tooling:** cosign, trivy (optional), jq
* **(Optional):** Tilt or Skaffold for inner‑loop dev; 1Password Connect or Vault for secrets

> macOS: `brew install go node pnpm kind helm cosign trivy jq`

---

## Quickstart (local dev in 10–15 min)

### 1) Clone & bootstrap

```bash
git clone git@github.com:madfam/enclii.git && cd enclii
make bootstrap    # installs hooks, pnpm deps, go workspaces, pre-commit
```

### 2) Spin up a local cluster

```bash
make kind-up          # creates kind cluster `enclii`
make infra-dev        # installs Ingress (NGINX), cert-manager, Prometheus, Loki, Tempo
make dns-dev          # dev DNS/hosts entries for *.dev.enclii.local
```

### 3) Run the platform

```bash
make run-switchyard   # control plane API on :8080
make run-ui           # web UI on http://localhost:3000
make run-reconcilers  # controllers watching the cluster
```

### 4) Try the CLI

```bash
make build-cli
./bin/enclii auth login           # opens browser (dev OIDC)
./bin/enclii init                 # scaffold a sample service
./bin/enclii up                   # build & deploy preview env
./bin/enclii deploy --env prod    # canary then promote
./bin/enclii logs api -f          # tail logs
```

> **Note:** Local dev uses a stub Lockbox (filesystem secrets) and self‑signed TLS. See `infra/dev/` for overrides. **Never** use the dev secrets backend in prod.

---

## Configuration

* **Workspace:** Go modules via `go.work`; Node via `pnpm-workspace.yaml`.
* **Environment:** `.env` at repo root for local only; production uses Lockbox/Vault.
* **Kube contexts:** `kind-enclii` for local; cloud contexts named `enclii-<region>`.

### Key environment variables

| Variable                | Purpose                                        | Example                 |
| ----------------------- | ---------------------------------------------- | ----------------------- |
| `ENCLII_DB_URL`         | Control plane DB (dev: sqlite; prod: Postgres) | `file:./dev.db`         |
| `ENCLII_REGISTRY`       | Container registry                             | `ghcr.io/madfam`        |
| `ENCLII_OIDC_ISSUER`    | Auth provider                                  | `http://localhost:5556` |
| `ENCLII_DEFAULT_REGION` | Default runtime region                         | `us-east`               |
| `ENCLII_LOG_LEVEL`      | log verbosity                                  | `info`                  |

Example: copy `.env.example` → `.env` and tweak for your setup.

---

## CLI overview (`enclii`)

```
$ enclii --help
Usage: enclii <command> [flags]

Commands:
  init              Scaffold a project from a template
  up                Build & deploy the current branch as a preview
  deploy            Promote last green build to an environment
  logs              Tail or fetch logs for a service
  ps                List services, versions, replicas, health
  secrets           Manage secrets (Lockbox)
  scale             Set min/max replicas (HPA)
  routes            Bind hosts/paths with TLS
  jobs              Run or manage cron jobs
  rollback          Revert a service to a previous release
  cost              Showback by project/env/service
  auth              Login / manage tokens
```

Common examples:

```bash
enclii deploy --env prod --strategy canary --wait
enclii secrets set API_KEY=... --service api --env prod
enclii routes add --host api.example.com --service api --env stage
```

---

## Development workflow

* **Inner loop:** run API/UI/controllers locally; use `kind` for data plane.
* **Builds:** `make build-all` compiles CLI + Go services + UI.
* **Tests:** `make test` runs unit tests; `make e2e` provisions a throwaway ns and runs smoke tests.
* **Linting:** `make lint` (golangci-lint, eslint, prettier).
* **Commits:** conventional commits; changelog generated on release.

---

## CI/CD

* **CI:** GitHub Actions; caches Go/Node deps; builds images with BuildKit; publishes SBOM; signs with cosign.
* **Preview envs:** `enclii up` from PRs; comment with URL.
* **Release:** `main` pushes trigger canary deploys to `stage`, then manual approval → `prod`.

Workflows live in `.github/workflows/`:

* `ci.yml` — build, test, lint
* `preview.yml` — PR previews
* `release.yml` — tag, build, sign, publish

---

## Observability (Signal)

* **Metrics:** Prometheus; service SLO panels in Grafana (`docs/dashboards`).
* **Logs:** Loki; `enclii logs` proxies queries per service/env.
* **Traces:** OpenTelemetry SDKs → Tempo.
* **Errors:** Sentry (optional; DSN via Lockbox).

---

## Security

* **Supply chain:** SBOM (CycloneDX), image signing (cosign), base image rotation every 30 days or on CVE.
* **Admission:** Kyverno/OPA policies; verify signatures; forbid latest tags in prod.
* **Secrets:** Lockbox (Vault/1Password) in prod; never commit secrets; CI uses short‑lived tokens.
* **Auth:** OIDC SSO; PATs are scoped and expire.

Responsible disclosure: [security@enclii.dev](mailto:security@enclii.dev)

---

## Environments

* **dev:** local kind cluster and sandbox cloud cluster
* **stage:** gated canary deploys; SLO enforced
* **prod:** audited changes; error‑budget policies

Domains follow `{service}.{project}.{env}.enclii.dev` by default; custom domains via `routes`.

---

## Roadmap

Milestones live in **Projects → Enclii**. High‑level:

* **Alpha:** control plane, CLI (init/up/deploy/logs), previews, TLS/DNS, single region
* **Beta:** KEDA autoscaling, cost showback, cron/jobs, volumes, RBAC, dashboards
* **GA:** multi‑region, policy‑as‑code gates, audit exports

See [`SOFTWARE_SPEC.md`](./SOFTWARE_SPEC.md) for acceptance criteria.

---

## Contributing

Internal only for now. Open a draft PR early; request a **DX review** for CLI/UX changes.
Run `make precommit` before pushing.

---

## Troubleshooting

* **Ingress 404 locally:** run `make infra-dev` and check that `ingress-nginx` pods are Ready.
* **TLS fails on preview:** in dev, self‑signed certs only; use `--insecure` CLI flag locally.
* **Cannot login:** ensure `ENCLII_OIDC_ISSUER` matches the dev IdP URL; `enclii auth logout` then retry.
* **Builds are slow:** enable BuildKit cache (`~/.cache/buildkit`) and `make builder-up`.

---

## License

Proprietary © Innovaciones MADFAM S.A.S. de C.V. All rights reserved.

---

## Acknowledgements

Inspired by the paved‑road philosophy and prior art in PaaS and IDP ecosystems. Names: **Switchyard, Conductor, Roundhouse, Junctions, Timetable, Lockbox, Signal, Waybill**.
