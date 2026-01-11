# Enclii Go SDK

Official Go SDK for the Enclii Platform API.

## Installation

```bash
go get github.com/madfam-org/enclii/packages/sdk-go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    enclii "github.com/madfam-org/enclii/packages/sdk-go/pkg/client"
)

func main() {
    // Create client with API token
    client := enclii.NewClient(
        enclii.WithAPIToken("enclii_xxx..."),
        enclii.WithBaseURL("https://api.enclii.dev"),
    )

    // List projects
    ctx := context.Background()
    projects, err := client.Projects.List(ctx)
    if err != nil {
        log.Fatal(err)
    }

    for _, p := range projects {
        fmt.Printf("Project: %s (%s)\n", p.Name, p.ID)
    }
}
```

## Authentication

### API Token

Get an API token from the Enclii dashboard or CLI:

```bash
# Generate token via CLI
enclii tokens create --name "my-ci-token" --scopes "read,deploy"
```

Use the token in your Go code:

```go
client := enclii.NewClient(
    enclii.WithAPIToken("enclii_xxx..."),
)
```

### Environment Variable

The SDK automatically reads `ENCLII_TOKEN`:

```bash
export ENCLII_TOKEN="enclii_xxx..."
```

```go
client := enclii.NewClient() // Reads ENCLII_TOKEN
```

## API Reference

### Projects

```go
// List all projects
projects, err := client.Projects.List(ctx)

// Get a specific project
project, err := client.Projects.Get(ctx, "proj_abc123")

// Create a project
project, err := client.Projects.Create(ctx, &enclii.CreateProjectRequest{
    Name: "my-project",
    Slug: "my-project",
})

// Delete a project
err := client.Projects.Delete(ctx, "proj_abc123")
```

### Services

```go
// List services in a project
services, err := client.Services.List(ctx, "proj_abc123")

// Get a specific service
service, err := client.Services.Get(ctx, "svc_xyz789")

// Create a service
service, err := client.Services.Create(ctx, "proj_abc123", &enclii.CreateServiceRequest{
    Name:    "api",
    GitRepo: "https://github.com/org/repo",
    BuildConfig: enclii.BuildConfig{
        Type: enclii.BuildTypeAuto,
    },
})

// Update a service
service, err := client.Services.Update(ctx, "svc_xyz789", &enclii.UpdateServiceRequest{
    AutoDeploy:       true,
    AutoDeployBranch: "main",
})

// Delete a service
err := client.Services.Delete(ctx, "svc_xyz789")
```

### Deployments

```go
// List deployments
deployments, err := client.Deployments.List(ctx, "svc_xyz789")

// Create a deployment
deployment, err := client.Deployments.Create(ctx, &enclii.CreateDeploymentRequest{
    ServiceID:     "svc_xyz789",
    EnvironmentID: "env_prod",
    ReleaseID:     "rel_abc123",
    Replicas:      3,
})

// Get deployment status
deployment, err := client.Deployments.Get(ctx, "dep_123")

// Rollback
deployment, err := client.Deployments.Rollback(ctx, "dep_123", &enclii.RollbackRequest{
    ToReleaseID: "rel_previous",
})
```

### Releases

```go
// List releases for a service
releases, err := client.Releases.List(ctx, "svc_xyz789")

// Get a specific release
release, err := client.Releases.Get(ctx, "rel_abc123")

// Trigger a build
release, err := client.Releases.Create(ctx, &enclii.CreateReleaseRequest{
    ServiceID: "svc_xyz789",
    GitSHA:    "a1b2c3d4e5f6",
})
```

### Environments

```go
// List environments
envs, err := client.Environments.List(ctx, "proj_abc123")

// Get environment
env, err := client.Environments.Get(ctx, "env_staging")

// Create environment
env, err := client.Environments.Create(ctx, "proj_abc123", &enclii.CreateEnvironmentRequest{
    Name: "staging",
})
```

### Environment Variables

```go
// List env vars for a service
vars, err := client.EnvVars.List(ctx, "svc_xyz789")

// Set an env var
envVar, err := client.EnvVars.Set(ctx, "svc_xyz789", &enclii.SetEnvVarRequest{
    Key:           "DATABASE_URL",
    Value:         "postgresql://...",
    IsSecret:      true,
    EnvironmentID: "env_prod", // nil for all environments
})

// Delete an env var
err := client.EnvVars.Delete(ctx, "var_123")
```

### Custom Domains

```go
// List domains
domains, err := client.Domains.List(ctx, "svc_xyz789")

// Add a domain
domain, err := client.Domains.Create(ctx, &enclii.CreateDomainRequest{
    ServiceID:     "svc_xyz789",
    EnvironmentID: "env_prod",
    Domain:        "api.example.com",
})

// Verify domain
verified, err := client.Domains.Verify(ctx, "dom_123")

// Delete domain
err := client.Domains.Delete(ctx, "dom_123")
```

### Logs

```go
// Fetch recent logs
logs, err := client.Logs.Fetch(ctx, "svc_xyz789", &enclii.LogsRequest{
    Since:       time.Now().Add(-1 * time.Hour),
    Tail:        100,
    Level:       "error",
    Environment: "env_prod",
})

// Stream logs (WebSocket)
logStream, err := client.Logs.Stream(ctx, "svc_xyz789", &enclii.LogsRequest{
    Environment: "env_prod",
})

for log := range logStream.Messages() {
    fmt.Printf("[%s] %s\n", log.Level, log.Message)
}
```

## Error Handling

The SDK returns typed errors for common scenarios:

```go
deployment, err := client.Deployments.Get(ctx, "dep_123")
if err != nil {
    switch e := err.(type) {
    case *enclii.NotFoundError:
        fmt.Printf("Deployment not found: %s\n", e.ResourceID)
    case *enclii.AuthError:
        fmt.Println("Authentication failed - check your token")
    case *enclii.RateLimitError:
        fmt.Printf("Rate limited - retry after %v\n", e.RetryAfter)
    case *enclii.ValidationError:
        fmt.Printf("Validation failed: %v\n", e.Errors)
    default:
        fmt.Printf("Unexpected error: %v\n", err)
    }
}
```

## Configuration Options

```go
client := enclii.NewClient(
    // Required: API authentication
    enclii.WithAPIToken("enclii_xxx..."),

    // Optional: Custom API URL
    enclii.WithBaseURL("https://api.enclii.dev"),

    // Optional: Custom HTTP client
    enclii.WithHTTPClient(&http.Client{
        Timeout: 30 * time.Second,
    }),

    // Optional: Request timeout
    enclii.WithTimeout(60 * time.Second),

    // Optional: Retry configuration
    enclii.WithRetry(3, time.Second),

    // Optional: Debug logging
    enclii.WithDebug(true),

    // Optional: Custom user agent
    enclii.WithUserAgent("my-app/1.0"),
)
```

## Pagination

List endpoints support pagination:

```go
// First page
projects, err := client.Projects.List(ctx,
    enclii.WithPage(1),
    enclii.WithPerPage(20),
)

// Next page
for projects.HasNextPage() {
    projects, err = projects.NextPage(ctx)
    if err != nil {
        break
    }

    for _, p := range projects.Items {
        fmt.Println(p.Name)
    }
}
```

## Webhook Handling

Verify and parse webhook payloads:

```go
import "github.com/madfam-org/enclii/packages/sdk-go/pkg/webhook"

func handleWebhook(w http.ResponseWriter, r *http.Request) {
    // Verify signature
    payload, err := webhook.Verify(r, "whsec_xxx...")
    if err != nil {
        http.Error(w, "Invalid signature", http.StatusUnauthorized)
        return
    }

    // Parse event
    event, err := webhook.Parse(payload)
    if err != nil {
        http.Error(w, "Invalid payload", http.StatusBadRequest)
        return
    }

    switch e := event.(type) {
    case *webhook.DeploymentCreated:
        fmt.Printf("Deployment started: %s\n", e.DeploymentID)
    case *webhook.DeploymentSucceeded:
        fmt.Printf("Deployment succeeded: %s\n", e.DeploymentID)
    case *webhook.DeploymentFailed:
        fmt.Printf("Deployment failed: %s - %s\n", e.DeploymentID, e.Error)
    case *webhook.BuildCompleted:
        fmt.Printf("Build completed: %s\n", e.ReleaseID)
    }

    w.WriteHeader(http.StatusOK)
}
```

## Examples

### CI/CD Integration

```go
// Deploy on successful CI build
func deployOnSuccess(ctx context.Context) error {
    client := enclii.NewClient()

    // Trigger build
    release, err := client.Releases.Create(ctx, &enclii.CreateReleaseRequest{
        ServiceID: os.Getenv("ENCLII_SERVICE_ID"),
        GitSHA:    os.Getenv("GITHUB_SHA"),
    })
    if err != nil {
        return fmt.Errorf("build failed: %w", err)
    }

    // Wait for build to complete
    release, err = client.Releases.WaitForReady(ctx, release.ID, 10*time.Minute)
    if err != nil {
        return fmt.Errorf("build did not complete: %w", err)
    }

    // Deploy to staging
    deployment, err := client.Deployments.Create(ctx, &enclii.CreateDeploymentRequest{
        ReleaseID:     release.ID,
        EnvironmentID: "env_staging",
    })
    if err != nil {
        return fmt.Errorf("deployment failed: %w", err)
    }

    fmt.Printf("Deployed to staging: %s\n", deployment.ID)
    return nil
}
```

### Programmatic Scaling

```go
// Scale based on queue depth
func autoScale(ctx context.Context, queueDepth int) error {
    client := enclii.NewClient()

    // Calculate desired replicas
    replicas := max(1, min(10, queueDepth/100))

    // Update deployment
    _, err := client.Deployments.Scale(ctx, "dep_123", replicas)
    if err != nil {
        return fmt.Errorf("scaling failed: %w", err)
    }

    log.Printf("Scaled to %d replicas\n", replicas)
    return nil
}
```

## Types Reference

See the [types package](./pkg/types/types.go) for all data structures:

- `Project` - Project resource
- `Environment` - Deployment environment
- `Service` - Deployable service
- `Release` - Built container image
- `Deployment` - Running instance
- `CustomDomain` - Domain mapping
- `EnvironmentVariable` - Configuration
- `Team` - User group
- `APIToken` - Programmatic access

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `go test ./...`
5. Submit a pull request

## License

Apache 2.0 - See [LICENSE](../../LICENSE) for details.
