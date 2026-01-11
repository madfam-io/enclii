# Enclii CLI

Command-line interface for the Enclii platform.

## Overview

The `enclii` CLI enables developers to:
- Deploy and manage services
- Stream logs and debug issues
- Configure domains and environments
- Manage teams and access

## Installation

### macOS (Homebrew)

```bash
brew install enclii/tap/enclii
```

### Linux

```bash
curl -sSL https://get.enclii.dev | bash
```

### Windows

```powershell
# Using scoop
scoop bucket add enclii https://github.com/madfam-org/scoop-enclii
scoop install enclii
```

### From Source

```bash
git clone https://github.com/madfam-org/enclii.git
cd enclii
make build-cli
./bin/enclii version
```

## Quick Start

```bash
# Authenticate
enclii login

# Initialize a service
cd my-app
enclii init

# Deploy
enclii deploy

# View logs
enclii logs my-app -f
```

## Project Structure

```
packages/cli/
├── cmd/
│   └── enclii/           # Main entry point
├── internal/
│   ├── cmd/              # Command implementations
│   │   ├── login.go      # Authentication commands
│   │   ├── deploy.go     # Deployment commands
│   │   ├── logs.go       # Log streaming
│   │   ├── services.go   # Service management
│   │   └── ...
│   ├── api/              # API client
│   ├── auth/             # OAuth/PKCE flow
│   ├── config/           # Configuration management
│   ├── output/           # Terminal output formatting
│   └── websocket/        # WebSocket client (logs)
└── docs/                 # CLI documentation
```

## Commands

See the [CLI Reference](../../docs/cli/README.md) for complete documentation.

| Command | Description |
|---------|-------------|
| `login` | Authenticate via SSO |
| `logout` | Clear credentials |
| `whoami` | Show current user |
| `init` | Initialize service config |
| `deploy` | Deploy a service |
| `ps` | List services |
| `logs` | Stream service logs |
| `rollback` | Rollback deployment |
| `services sync` | Sync configuration |
| `local` | Local development |
| `version` | Show version info |

## Development

### Prerequisites

- Go 1.22+

### Building

```bash
# Build for current platform
go build -o bin/enclii ./cmd/enclii

# Build all platforms
make build-cli-all

# Build with version info
go build -ldflags "-X main.version=v0.5.0" -o bin/enclii ./cmd/enclii
```

### Testing

```bash
# Run tests
go test ./...

# Run with coverage
go test -coverprofile=coverage.out ./...

# Integration tests
go test -tags=integration ./...
```

### Linting

```bash
# Run golangci-lint
golangci-lint run
```

## Configuration

The CLI stores configuration in `~/.enclii/config.yaml`:

```yaml
api_url: https://api.enclii.dev
auth:
  token: eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...
  refresh_token: ...
  expires_at: 2025-02-01T00:00:00Z
defaults:
  project: proj_abc123
  environment: staging
output:
  format: table
  color: true
```

## Authentication Flow

The CLI uses OAuth 2.0 with PKCE:

```
1. CLI generates code_verifier and code_challenge
2. Opens browser to auth.madfam.io/authorize
3. User authenticates with Janua SSO
4. Janua redirects to localhost callback
5. CLI exchanges code for tokens
6. Tokens stored in config file
```

## API Client

The CLI uses a generated API client:

```go
// internal/api/client.go
type Client struct {
    BaseURL    string
    HTTPClient *http.Client
    Token      string
}

func (c *Client) CreateDeployment(ctx context.Context, req *CreateDeploymentRequest) (*Deployment, error) {
    // ...
}
```

## Output Formatting

The CLI supports multiple output formats:

```go
// internal/output/formatter.go
type Formatter interface {
    Format(data interface{}) string
}

type TableFormatter struct{}
type JSONFormatter struct{}
type YAMLFormatter struct{}
```

Usage:
```bash
enclii ps -o json
enclii ps -o yaml
enclii ps -o table  # default
```

## WebSocket Log Streaming

Real-time logs use WebSocket:

```go
// internal/websocket/logs.go
func StreamLogs(ctx context.Context, serviceID string, opts LogsOptions) (<-chan LogEntry, error) {
    conn, _, err := websocket.DefaultDialer.Dial(wsURL, headers)
    if err != nil {
        return nil, err
    }

    ch := make(chan LogEntry)
    go func() {
        defer close(ch)
        for {
            _, msg, err := conn.ReadMessage()
            if err != nil {
                return
            }
            var entry LogEntry
            json.Unmarshal(msg, &entry)
            ch <- entry
        }
    }()

    return ch, nil
}
```

## Release Process

1. Update version in `version.go`
2. Create git tag: `git tag v0.5.0`
3. Push tag: `git push origin v0.5.0`
4. GitHub Actions builds and releases

## Related Components

- **[Switchyard API](../../apps/switchyard-api/)** - Backend API
- **[Switchyard UI](../../apps/switchyard-ui/)** - Web dashboard
- **[SDK Go](../sdk-go/)** - Go SDK

## License

Apache 2.0 - See [LICENSE](../../LICENSE)
