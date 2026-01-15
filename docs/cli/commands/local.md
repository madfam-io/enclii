# enclii local

Local development environment commands.

## Synopsis

```bash
enclii local <subcommand> [flags]
```

## Description

The `local` command group manages local development environments. It provisions local infrastructure (databases, caches, message queues) and runs your services in containers that mirror production behavior.

## Subcommands

| Subcommand | Description |
|------------|-------------|
| [`up`](#up) | Start local development environment |
| [`down`](#down) | Stop and remove local environment |
| [`status`](#status) | Show status of local services |
| [`logs`](#logs) | View logs from local services |
| [`infra`](#infra) | Manage local infrastructure |

---

## up

Start the local development environment.

### Synopsis
```bash
enclii local up [flags]
```

### Flags
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--build`, `-b` | bool | `false` | Rebuild containers before starting |
| `--detach`, `-d` | bool | `true` | Run in background |
| `--infra-only` | bool | `false` | Start only infrastructure (DB, Redis) |
| `--service` | string | all | Start specific service only |

### Examples

```bash
# Start all services
enclii local up

# Rebuild and start
enclii local up --build

# Start only infrastructure
enclii local up --infra-only

# Start specific service
enclii local up --service api
```

**Output:**
```
Starting local environment...

Infrastructure:
  ✓ postgres     localhost:5432
  ✓ redis        localhost:6379

Services:
  ✓ api          http://localhost:8080
  ✓ web          http://localhost:3000

Local environment ready!
  Dashboard: http://localhost:4201
  API:       http://localhost:8080
  Logs:      enclii local logs -f
```

---

## down

Stop and remove the local development environment.

### Synopsis
```bash
enclii local down [flags]
```

### Flags
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--volumes`, `-v` | bool | `false` | Also remove persistent volumes (data) |
| `--all` | bool | `false` | Remove all resources including networks |

### Examples

```bash
# Stop services (keep data)
enclii local down

# Stop and remove all data
enclii local down --volumes

# Full cleanup
enclii local down --all --volumes
```

---

## status

Show status of local services.

### Synopsis
```bash
enclii local status [flags]
```

### Flags
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--output`, `-o` | string | `table` | Output format: `table`, `json` |

### Examples

```bash
enclii local status
```

**Output:**
```
SERVICE     STATUS    PORTS                 CPU    MEMORY
api         running   8080:8080             12%    156Mi
web         running   3000:3000             8%     98Mi
postgres    running   5432:5432             2%     89Mi
redis       running   6379:6379             1%     12Mi
```

---

## logs

View logs from local services.

### Synopsis
```bash
enclii local logs [service] [flags]
```

### Flags
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--follow`, `-f` | bool | `false` | Stream logs |
| `--tail`, `-n` | int | `100` | Number of lines |
| `--timestamps` | bool | `true` | Show timestamps |

### Examples

```bash
# All service logs
enclii local logs -f

# Specific service
enclii local logs api -f

# Last 50 lines
enclii local logs --tail 50
```

---

## infra

Manage local infrastructure services.

### Synopsis
```bash
enclii local infra <action> [flags]
```

### Actions
| Action | Description |
|--------|-------------|
| `list` | List available infrastructure services |
| `add` | Add infrastructure service |
| `remove` | Remove infrastructure service |

### Examples

```bash
# List available infrastructure
enclii local infra list
```

**Output:**
```
AVAILABLE INFRASTRUCTURE:
  postgres    PostgreSQL database (default: enabled)
  redis       Redis cache (default: enabled)
  rabbitmq    RabbitMQ message queue
  minio       S3-compatible object storage
  mailhog     Email testing server
```

```bash
# Add RabbitMQ
enclii local infra add rabbitmq

# Remove Redis
enclii local infra remove redis
```

## Local Configuration

The local environment reads from `.enclii/local.yaml`:

```yaml
# Infrastructure
infrastructure:
  postgres:
    enabled: true
    port: 5432
    database: app_dev
  redis:
    enabled: true
    port: 6379

# Service overrides
services:
  api:
    env:
      - name: DEBUG
        value: "true"
      - name: LOG_LEVEL
        value: debug
    ports:
      - 8080:8080
      - 9229:9229  # Debugger
```

## See Also

- [`enclii init`](./init.md) - Initialize service configuration
- [`enclii deploy`](./deploy.md) - Deploy to remote environments
