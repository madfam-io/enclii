# enclii ps

List services and their deployment status.

## Synopsis

```bash
enclii ps [flags]
```

## Description

The `ps` command displays the status of services in your project. Shows running instances, health status, resource usage, and deployment information.

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--env`, `-e` | string | all | Filter by environment |
| `--all`, `-a` | bool | `false` | Show all services (including stopped) |
| `--output`, `-o` | string | `table` | Output format: `table`, `json`, `yaml`, `wide` |
| `--watch`, `-w` | bool | `false` | Continuously refresh output |

## Examples

### List All Services
```bash
enclii ps
```

**Output:**
```
NAME          ENV         STATUS    INSTANCES   CPU    MEMORY   AGE
api           production  running   3/3         45%    312Mi    5d
api           staging     running   1/1         12%    156Mi    2h
web           production  running   2/2         23%    245Mi    5d
worker        production  running   1/1         67%    890Mi    1d
```

### Filter by Environment
```bash
enclii ps --env production
```

### Wide Output (More Details)
```bash
enclii ps -o wide
```

**Output:**
```
NAME   ENV         STATUS   INSTANCES   CPU   MEMORY   RELEASE      STRATEGY   URL                      AGE
api    production  running  3/3         45%   312Mi    rel_abc123   rolling    https://api.acme.com     5d
web    production  running  2/2         23%   245Mi    rel_def456   blue-green https://app.acme.com     5d
```

### JSON Output
```bash
enclii ps -o json
```

**Output:**
```json
{
  "services": [
    {
      "name": "api",
      "environment": "production",
      "status": "running",
      "instances": {"ready": 3, "desired": 3},
      "resources": {"cpu": "45%", "memory": "312Mi"},
      "release": "rel_abc123",
      "url": "https://api.acme.com",
      "age": "5d"
    }
  ]
}
```

### Watch Mode
```bash
enclii ps --watch
# Refreshes every 2 seconds
```

### Show Stopped Services
```bash
enclii ps --all
```

## Status Values

| Status | Description |
|--------|-------------|
| `running` | All instances healthy |
| `degraded` | Some instances unhealthy |
| `deploying` | Deployment in progress |
| `scaling` | Scaling operation in progress |
| `stopped` | Service manually stopped |
| `failed` | Deployment failed |

## See Also

- [`enclii logs`](./logs.md) - View service logs
- [`enclii deploy`](./deploy.md) - Deploy a service
- [`enclii rollback`](./rollback.md) - Rollback deployment
