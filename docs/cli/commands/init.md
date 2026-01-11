# enclii init

Initialize a new service configuration.

## Synopsis

```bash
enclii init [flags]
```

## Description

The `init` command scaffolds a new Enclii service configuration file (`enclii.yaml`) in the current directory. It auto-detects the project type (Node.js, Go, Python, etc.) and generates appropriate defaults for build, runtime, and health check settings.

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--name`, `-n` | string | directory name | Service name |
| `--template`, `-t` | string | auto-detect | Template: `node`, `go`, `python`, `docker`, `static` |
| `--port`, `-p` | int | auto-detect | Service port |
| `--force`, `-f` | bool | `false` | Overwrite existing `enclii.yaml` |
| `--no-detect` | bool | `false` | Skip auto-detection, use minimal config |

## Examples

### Auto-Detect Configuration
```bash
cd my-nodejs-app
enclii init
```

**Output:**
```
Detected: Node.js application (package.json found)
Created: enclii.yaml

Service: my-nodejs-app
Type:    http
Port:    3000
Build:   nixpacks (auto-detected)

Next steps:
  1. Review enclii.yaml
  2. Run: enclii deploy --env preview
```

### Specify Template
```bash
enclii init --template go --name api-service --port 8080
```

### Minimal Configuration
```bash
enclii init --no-detect --name worker-service
```

## Generated Configuration

The command creates `enclii.yaml` with the following structure:

```yaml
apiVersion: enclii.dev/v1
kind: Service
metadata:
  name: my-nodejs-app
spec:
  type: http
  port: 3000

  build:
    type: nixpacks
    # Or: dockerfile: ./Dockerfile

  runtime:
    instances: 1
    resources:
      cpu: "0.5"
      memory: "512Mi"

  healthCheck:
    path: /health
    interval: 30s
    timeout: 5s

  env:
    - name: NODE_ENV
      value: production
```

## Auto-Detection Logic

| File Detected | Template Used | Default Port |
|---------------|---------------|--------------|
| `package.json` | node | 3000 |
| `go.mod` | go | 8080 |
| `requirements.txt` or `pyproject.toml` | python | 8000 |
| `Dockerfile` | docker | 8080 |
| `index.html` | static | 80 |

## See Also

- [Service Specification Reference](../../reference/service-spec.md)
- [`enclii deploy`](./deploy.md) - Deploy the service
- [`enclii services sync`](./services-sync.md) - Sync configuration
