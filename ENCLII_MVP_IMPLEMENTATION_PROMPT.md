# Enclii MVP Implementation Prompt
## Complete Vercel/Railway Feature Parity Implementation Guide

> **Purpose**: This document provides a comprehensive, step-by-step prompt for a Software Engineering agent to implement all remaining features needed to bring Enclii to MVP feature parity with Vercel and Railway.

---

## Executive Summary

| Current Status | Target | Gap |
|----------------|--------|-----|
| 55% MVP Complete | 100% MVP | ~290 hours of work |
| Single-service deploy working | Multi-service monorepo deploy | Core feature gap |
| CLI fully functional | Full UI parity | UI completion needed |
| Auth + RBAC working | Team management UI | Feature gap |

**Goal**: Enable full deployment of monorepo projects (like Dhanam) via Enclii's web UI with automatic service detection, coordinated multi-service deployment, and Cloudflare domain/tunnel configuration.

---

## Project Context

### Repository Structure
```
/Users/aldoruizluna/labspace/enclii/
├── apps/
│   ├── switchyard-api/          # Go REST API (control plane) - 95% complete
│   ├── switchyard-ui/           # Next.js 14 web UI - 50% complete
│   ├── roundhouse/              # Build worker service - ready
│   └── reconcilers/             # K8s operators - partial
├── packages/
│   ├── cli/                     # Go CLI tool - 100% complete
│   └── sdk-go/                  # Go SDK types - ready
├── docs/design/                 # Design documents (use as specs)
│   ├── MONOREPO_PROJECT_MODEL.md    # Monorepo support spec (795 lines)
│   └── CLOUDFLARE_TUNNEL_UI.md      # Cloudflare integration spec (607 lines)
└── infra/                       # Terraform + K8s configs
```

### Tech Stack
- **API**: Go 1.21+, Chi router, PostgreSQL, Redis
- **UI**: Next.js 14, TypeScript, React 18, Tailwind CSS, shadcn/ui
- **CLI**: Go, Cobra framework
- **Infrastructure**: Kubernetes, Cloudflare Tunnels, cert-manager

### What's Already Working ✅
- User authentication (JWT + OIDC via Janua)
- Project and service CRUD
- Single-service build and deployment
- CLI commands (init, deploy, logs, ps, rollback)
- Basic UI pages (dashboard, projects, services, deployments)
- Database schema for core entities
- Kubernetes integration for deployments
- GitHub integration for repository listing

---

# PHASE 1: Deployment Groups & Multi-Service Deploy
**Estimated Time**: 2-3 weeks | **Priority**: 🔴 CRITICAL

## Task 1.1: Database Migration for Deployment Groups

**Objective**: Create database tables for coordinated multi-service deployments.

**File to Create**: `apps/switchyard-api/internal/db/migrations/010_deployment_groups.up.sql`

**Reference**: `docs/design/MONOREPO_PROJECT_MODEL.md` lines 100-118

```sql
-- Create this migration file with:

-- Deployment groups for atomic multi-service deployments
CREATE TABLE deployment_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    name VARCHAR(255),
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    -- 'pending', 'in_progress', 'deploying', 'succeeded', 'failed', 'rolled_back'
    strategy VARCHAR(50) NOT NULL DEFAULT 'dependency_ordered',
    -- 'parallel', 'dependency_ordered', 'sequential'
    triggered_by VARCHAR(255), -- 'webhook', 'manual', 'promotion'
    git_sha VARCHAR(40),
    pr_url TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Link deployments to their group
ALTER TABLE deployments ADD COLUMN group_id UUID REFERENCES deployment_groups(id);
ALTER TABLE deployments ADD COLUMN deploy_order INTEGER DEFAULT 0;

-- Service dependencies within a project
CREATE TABLE service_dependencies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    depends_on_service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    dependency_type VARCHAR(50) NOT NULL DEFAULT 'runtime',
    -- 'runtime', 'build', 'data'
    created_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT unique_dependency UNIQUE(service_id, depends_on_service_id),
    CONSTRAINT no_self_dependency CHECK(service_id != depends_on_service_id)
);

-- Indexes
CREATE INDEX idx_deployment_groups_project ON deployment_groups(project_id);
CREATE INDEX idx_deployment_groups_status ON deployment_groups(status);
CREATE INDEX idx_deployments_group ON deployments(group_id);
CREATE INDEX idx_service_dependencies_service ON service_dependencies(service_id);
CREATE INDEX idx_service_dependencies_depends_on ON service_dependencies(depends_on_service_id);

-- Triggers
CREATE TRIGGER update_deployment_groups_updated_at
    BEFORE UPDATE ON deployment_groups
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

**Down Migration**: `010_deployment_groups.down.sql`
```sql
DROP TABLE IF EXISTS service_dependencies;
ALTER TABLE deployments DROP COLUMN IF EXISTS deploy_order;
ALTER TABLE deployments DROP COLUMN IF EXISTS group_id;
DROP TABLE IF EXISTS deployment_groups;
```

**Validation**:
```bash
cd apps/switchyard-api
go run cmd/main.go migrate up
# Verify tables exist:
psql $DATABASE_URL -c "\d deployment_groups"
psql $DATABASE_URL -c "\d service_dependencies"
```

---

## Task 1.2: Deployment Group Repository

**Objective**: Create data access layer for deployment groups.

**File to Create**: `apps/switchyard-api/internal/db/deployment_groups.go`

**Implementation**:
```go
package db

import (
    "context"
    "database/sql"
    "time"

    "github.com/google/uuid"
)

type DeploymentGroup struct {
    ID            uuid.UUID      `db:"id"`
    ProjectID     uuid.UUID      `db:"project_id"`
    EnvironmentID uuid.UUID      `db:"environment_id"`
    Name          sql.NullString `db:"name"`
    Status        string         `db:"status"`
    Strategy      string         `db:"strategy"`
    TriggeredBy   sql.NullString `db:"triggered_by"`
    GitSHA        sql.NullString `db:"git_sha"`
    PRURL         sql.NullString `db:"pr_url"`
    StartedAt     sql.NullTime   `db:"started_at"`
    CompletedAt   sql.NullTime   `db:"completed_at"`
    ErrorMessage  sql.NullString `db:"error_message"`
    CreatedAt     time.Time      `db:"created_at"`
    UpdatedAt     time.Time      `db:"updated_at"`
}

type ServiceDependency struct {
    ID                 uuid.UUID `db:"id"`
    ServiceID          uuid.UUID `db:"service_id"`
    DependsOnServiceID uuid.UUID `db:"depends_on_service_id"`
    DependencyType     string    `db:"dependency_type"`
    CreatedAt          time.Time `db:"created_at"`
}

type DeploymentGroupRepository interface {
    Create(ctx context.Context, group *DeploymentGroup) error
    GetByID(ctx context.Context, id uuid.UUID) (*DeploymentGroup, error)
    GetByProject(ctx context.Context, projectID, envID uuid.UUID) ([]*DeploymentGroup, error)
    UpdateStatus(ctx context.Context, id uuid.UUID, status string, errorMsg *string) error
    GetDeploymentsInGroup(ctx context.Context, groupID uuid.UUID) ([]*Deployment, error)
}

type ServiceDependencyRepository interface {
    Create(ctx context.Context, dep *ServiceDependency) error
    GetByService(ctx context.Context, serviceID uuid.UUID) ([]*ServiceDependency, error)
    GetDependents(ctx context.Context, serviceID uuid.UUID) ([]*ServiceDependency, error)
    Delete(ctx context.Context, serviceID, dependsOnID uuid.UUID) error
    GetProjectDependencyGraph(ctx context.Context, projectID uuid.UUID) (map[uuid.UUID][]uuid.UUID, error)
}

// Implement all methods following existing patterns in:
// - apps/switchyard-api/internal/db/projects.go
// - apps/switchyard-api/internal/db/deployments.go
```

**Reference existing code patterns in**:
- `apps/switchyard-api/internal/db/projects.go`
- `apps/switchyard-api/internal/db/deployments.go`

---

## Task 1.3: Deployment Group Service Layer

**Objective**: Implement business logic for multi-service deployment orchestration.

**File to Create**: `apps/switchyard-api/internal/services/deployment_groups.go`

**Key Functions to Implement**:

```go
package services

import (
    "context"
    "sort"

    "github.com/google/uuid"
)

type DeploymentGroupService struct {
    groupRepo      db.DeploymentGroupRepository
    depRepo        db.ServiceDependencyRepository
    deploymentSvc  *DeploymentService
    projectRepo    db.ProjectRepository
    serviceRepo    db.ServiceRepository
}

// CreateGroupDeployment initiates a coordinated multi-service deployment
// Reference: docs/design/MONOREPO_PROJECT_MODEL.md lines 671-729
func (s *DeploymentGroupService) CreateGroupDeployment(
    ctx context.Context,
    projectID uuid.UUID,
    environmentID uuid.UUID,
    serviceIDs []uuid.UUID, // nil = all project services
    triggeredBy string,
    gitSHA string,
    prURL string,
) (*DeploymentGroup, error) {
    // 1. If serviceIDs is nil, get all services for project
    // 2. Build dependency graph
    // 3. Topological sort for deployment order
    // 4. Create DeploymentGroup record
    // 5. Create individual Deployment records with group_id and deploy_order
    // 6. Start deployment orchestration (async)
    // 7. Return group with pending status
}

// TopologicalSort returns services in dependency order
// Reference: docs/design/MONOREPO_PROJECT_MODEL.md lines 684-707
func (s *DeploymentGroupService) TopologicalSort(
    ctx context.Context,
    serviceIDs []uuid.UUID,
) ([][]uuid.UUID, error) {
    // Returns layers: [[no-deps], [depends-on-layer-0], [depends-on-layer-1], ...]
    // Services in same layer can be deployed in parallel

    // Algorithm:
    // 1. Build adjacency list from service_dependencies
    // 2. Calculate in-degree for each service
    // 3. Start with services that have in-degree 0
    // 4. Remove edges, repeat until all processed
    // 5. Detect cycles (error if any)
}

// ExecuteGroupDeployment runs the actual deployment orchestration
func (s *DeploymentGroupService) ExecuteGroupDeployment(
    ctx context.Context,
    groupID uuid.UUID,
) error {
    // 1. Update group status to 'in_progress'
    // 2. Get sorted deployment order
    // 3. For each layer:
    //    a. Deploy all services in layer (parallel within layer)
    //    b. Wait for all to succeed
    //    c. If any fails, trigger rollback for completed services
    // 4. Update group status to 'succeeded' or 'failed'
}

// RollbackGroup rolls back all deployments in a group
func (s *DeploymentGroupService) RollbackGroup(
    ctx context.Context,
    groupID uuid.UUID,
) error {
    // 1. Get all deployments in group
    // 2. Rollback in reverse order
    // 3. Update group status to 'rolled_back'
}
```

**Reference**:
- Existing deployment service: `apps/switchyard-api/internal/services/deployments.go`
- Design spec: `docs/design/MONOREPO_PROJECT_MODEL.md` lines 671-729

---

## Task 1.4: Deployment Group API Handlers

**Objective**: Create REST API endpoints for deployment group operations.

**File to Modify**: `apps/switchyard-api/internal/api/deployment_handlers.go`

**New Endpoints to Add**:

```go
// Add to existing deployment_handlers.go or create deployment_group_handlers.go

// POST /v1/projects/{slug}/deployments
// Triggers a group deployment for all (or specified) services
func (h *Handler) CreateProjectDeployment(w http.ResponseWriter, r *http.Request) {
    // Request body:
    // {
    //   "environment_id": "uuid",
    //   "service_ids": ["uuid1", "uuid2"], // optional, nil = all services
    //   "triggered_by": "manual|webhook|promotion",
    //   "git_sha": "abc123",
    //   "pr_url": "https://github.com/..."
    // }

    // 1. Validate project access
    // 2. Call DeploymentGroupService.CreateGroupDeployment
    // 3. Return group with status
}

// GET /v1/projects/{slug}/deployments
// List deployment groups for a project
func (h *Handler) ListProjectDeployments(w http.ResponseWriter, r *http.Request) {
    // Query params: environment_id, status, limit, offset
}

// GET /v1/projects/{slug}/deployments/{group_id}
// Get deployment group status with individual deployment statuses
func (h *Handler) GetProjectDeployment(w http.ResponseWriter, r *http.Request) {
    // Return:
    // {
    //   "group": { ... },
    //   "deployments": [
    //     { "service_name": "api", "status": "succeeded", "deploy_order": 0 },
    //     { "service_name": "web", "status": "in_progress", "deploy_order": 1 }
    //   ]
    // }
}

// POST /v1/projects/{slug}/deployments/{group_id}/rollback
// Rollback entire deployment group
func (h *Handler) RollbackProjectDeployment(w http.ResponseWriter, r *http.Request) {
    // 1. Validate access
    // 2. Call DeploymentGroupService.RollbackGroup
    // 3. Return updated group status
}
```

**Add routes in**: `apps/switchyard-api/internal/api/handlers.go` (router setup)

```go
// Add to router setup:
r.Route("/v1/projects/{slug}/deployments", func(r chi.Router) {
    r.Post("/", h.CreateProjectDeployment)
    r.Get("/", h.ListProjectDeployments)
    r.Get("/{groupID}", h.GetProjectDeployment)
    r.Post("/{groupID}/rollback", h.RollbackProjectDeployment)
})
```

---

## Task 1.5: Service Dependencies API

**Objective**: API to manage service dependencies within a project.

**File to Create/Modify**: `apps/switchyard-api/internal/api/service_handlers.go`

**New Endpoints**:

```go
// GET /v1/services/{id}/dependencies
// List services this service depends on
func (h *Handler) GetServiceDependencies(w http.ResponseWriter, r *http.Request) {}

// POST /v1/services/{id}/dependencies
// Add a dependency
// Body: { "depends_on_service_id": "uuid", "dependency_type": "runtime" }
func (h *Handler) AddServiceDependency(w http.ResponseWriter, r *http.Request) {}

// DELETE /v1/services/{id}/dependencies/{dep_service_id}
// Remove a dependency
func (h *Handler) RemoveServiceDependency(w http.ResponseWriter, r *http.Request) {}

// GET /v1/projects/{slug}/dependency-graph
// Get full project dependency graph (for visualization)
func (h *Handler) GetProjectDependencyGraph(w http.ResponseWriter, r *http.Request) {
    // Return: { "nodes": [...], "edges": [...] }
}
```

---

## Task 1.6: Unit Tests for Deployment Groups

**File to Create**: `apps/switchyard-api/internal/services/deployment_groups_test.go`

**Test Cases**:
```go
func TestTopologicalSort_NoDependencies(t *testing.T) {
    // All services in one layer
}

func TestTopologicalSort_LinearChain(t *testing.T) {
    // A -> B -> C should return [[A], [B], [C]]
}

func TestTopologicalSort_Diamond(t *testing.T) {
    // A -> B, A -> C, B -> D, C -> D
    // Should return [[A], [B, C], [D]]
}

func TestTopologicalSort_CycleDetection(t *testing.T) {
    // A -> B -> C -> A should error
}

func TestExecuteGroupDeployment_AllSucceed(t *testing.T) {}
func TestExecuteGroupDeployment_PartialFailure(t *testing.T) {}
func TestRollbackGroup(t *testing.T) {}
```

---

# PHASE 2: Monorepo Service Detection & Import
**Estimated Time**: 2 weeks | **Priority**: 🔴 CRITICAL

## Task 2.1: Repository Analysis Endpoint

**Objective**: Implement automatic service detection from GitHub repositories.

**File to Modify**: `apps/switchyard-api/internal/api/integrations_handlers.go`

**Reference**: `docs/design/MONOREPO_PROJECT_MODEL.md` lines 150-250

```go
// POST /v1/integrations/github/repos/{owner}/{repo}/analyze
func (h *Handler) AnalyzeRepository(w http.ResponseWriter, r *http.Request) {
    owner := chi.URLParam(r, "owner")
    repo := chi.URLParam(r, "repo")

    // Query params:
    // - branch: default "main"
    // - app_path: optional, analyze specific subdirectory

    // 1. Fetch repository tree from GitHub API
    // 2. Scan for service indicators
    // 3. Return detected services
}

// Response structure:
type AnalysisResult struct {
    MonorepoDetected bool                `json:"monorepo_detected"`
    MonorepoTool     string              `json:"monorepo_tool"` // "turborepo", "nx", "lerna", "pnpm", "none"
    Services         []DetectedService   `json:"services"`
    SharedPaths      []string            `json:"shared_paths"` // packages/, libs/, etc.
}

type DetectedService struct {
    Name           string   `json:"name"`
    AppPath        string   `json:"app_path"`        // "apps/api", "services/web"
    Runtime        string   `json:"runtime"`         // "nodejs", "python", "go", "docker"
    Framework      string   `json:"framework"`       // "nextjs", "fastapi", "gin"
    Port           int      `json:"port"`            // detected from config
    BuildCommand   string   `json:"build_command"`   // detected from package.json/Dockerfile
    StartCommand   string   `json:"start_command"`
    Confidence     float64  `json:"confidence"`      // 0.0-1.0
    DetectionNotes []string `json:"detection_notes"` // why we detected this
}
```

## Task 2.2: Service Detection Logic

**File to Create**: `apps/switchyard-api/internal/services/analyzer.go`

**Detection Algorithm**:

```go
package services

type RepositoryAnalyzer struct {
    githubClient *github.Client
}

// AnalyzeRepository scans a GitHub repo for deployable services
func (a *RepositoryAnalyzer) AnalyzeRepository(
    ctx context.Context,
    owner, repo, branch string,
) (*AnalysisResult, error) {

    // 1. Get repository tree
    tree, err := a.githubClient.GetTree(ctx, owner, repo, branch)

    // 2. Detect monorepo tool
    monorepoTool := a.detectMonorepoTool(tree)

    // 3. Find service directories
    serviceDirs := a.findServiceDirectories(tree, monorepoTool)

    // 4. Analyze each service directory
    services := make([]DetectedService, 0)
    for _, dir := range serviceDirs {
        svc := a.analyzeServiceDirectory(ctx, owner, repo, branch, dir)
        if svc != nil {
            services = append(services, *svc)
        }
    }

    return &AnalysisResult{
        MonorepoDetected: len(services) > 1 || monorepoTool != "none",
        MonorepoTool:     monorepoTool,
        Services:         services,
    }, nil
}

// detectMonorepoTool checks for monorepo configuration files
func (a *RepositoryAnalyzer) detectMonorepoTool(tree []TreeEntry) string {
    for _, entry := range tree {
        switch entry.Path {
        case "turbo.json":
            return "turborepo"
        case "nx.json":
            return "nx"
        case "lerna.json":
            return "lerna"
        case "pnpm-workspace.yaml":
            return "pnpm"
        }
    }
    return "none"
}

// findServiceDirectories locates potential service directories
func (a *RepositoryAnalyzer) findServiceDirectories(tree []TreeEntry, tool string) []string {
    dirs := []string{}

    // Common patterns
    patterns := []string{
        "apps/*/Dockerfile",
        "apps/*/package.json",
        "services/*/Dockerfile",
        "packages/*/Dockerfile",
        "*/Dockerfile",  // root-level services
    }

    // Match patterns and extract directories
    for _, entry := range tree {
        for _, pattern := range patterns {
            if matchGlob(entry.Path, pattern) {
                dir := extractServiceDir(entry.Path)
                if !contains(dirs, dir) {
                    dirs = append(dirs, dir)
                }
            }
        }
    }

    return dirs
}

// analyzeServiceDirectory determines service type and configuration
func (a *RepositoryAnalyzer) analyzeServiceDirectory(
    ctx context.Context,
    owner, repo, branch, dir string,
) *DetectedService {

    files := a.listDirectory(ctx, owner, repo, branch, dir)

    svc := &DetectedService{
        Name:       filepath.Base(dir),
        AppPath:    dir,
        Confidence: 0.5,
    }

    // Check for Dockerfile (highest confidence)
    if hasFile(files, "Dockerfile") {
        svc.Runtime = "docker"
        svc.Confidence = 0.95
        svc.DetectionNotes = append(svc.DetectionNotes, "Found Dockerfile")
        // Parse Dockerfile for EXPOSE, CMD
        dockerfile := a.getFileContent(ctx, owner, repo, branch, dir+"/Dockerfile")
        svc.Port = parseExpose(dockerfile)
        svc.StartCommand = parseCMD(dockerfile)
    }

    // Check for package.json (Node.js)
    if hasFile(files, "package.json") {
        pkg := a.getPackageJSON(ctx, owner, repo, branch, dir)
        svc.Runtime = "nodejs"
        svc.Confidence = max(svc.Confidence, 0.85)

        // Detect framework
        if hasDep(pkg, "next") {
            svc.Framework = "nextjs"
            svc.Port = 3000
            svc.BuildCommand = "npm run build"
            svc.StartCommand = "npm start"
        } else if hasDep(pkg, "express") || hasDep(pkg, "fastify") {
            svc.Framework = "express"
            svc.Port = pkg.Scripts.Start.detectPort() // parse "node server.js" etc
        }
    }

    // Check for requirements.txt / pyproject.toml (Python)
    if hasFile(files, "requirements.txt") || hasFile(files, "pyproject.toml") {
        svc.Runtime = "python"
        svc.Confidence = max(svc.Confidence, 0.85)

        if hasFile(files, "main.py") && containsUvicorn(files) {
            svc.Framework = "fastapi"
            svc.Port = 8000
            svc.StartCommand = "uvicorn main:app --host 0.0.0.0"
        }
    }

    // Check for go.mod (Go)
    if hasFile(files, "go.mod") {
        svc.Runtime = "go"
        svc.Confidence = max(svc.Confidence, 0.85)
        svc.BuildCommand = "go build -o app ."
        svc.StartCommand = "./app"
    }

    return svc
}
```

---

## Task 2.3: Monorepo Import Wizard UI - Step 1 (Repository Selection)

**File to Create**: `apps/switchyard-ui/app/(protected)/projects/import/page.tsx`

**Reference**: `docs/design/MONOREPO_PROJECT_MODEL.md` lines 444-475

```tsx
'use client'

import { useState, useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Loader2, GitBranch, FolderSearch } from 'lucide-react'

interface Repository {
  id: number
  full_name: string
  name: string
  owner: { login: string }
  default_branch: string
  private: boolean
}

export default function ImportProjectPage() {
  const router = useRouter()
  const [repos, setRepos] = useState<Repository[]>([])
  const [selectedRepo, setSelectedRepo] = useState<Repository | null>(null)
  const [branch, setBranch] = useState('')
  const [branches, setBranches] = useState<string[]>([])
  const [analyzing, setAnalyzing] = useState(false)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetchRepositories()
  }, [])

  const fetchRepositories = async () => {
    const res = await fetch('/api/integrations/github/repos')
    const data = await res.json()
    setRepos(data.repositories || [])
    setLoading(false)
  }

  const fetchBranches = async (owner: string, repo: string) => {
    const res = await fetch(`/api/integrations/github/repos/${owner}/${repo}/branches`)
    const data = await res.json()
    setBranches(data.branches || [])
  }

  const handleRepoSelect = (repoFullName: string) => {
    const repo = repos.find(r => r.full_name === repoFullName)
    if (repo) {
      setSelectedRepo(repo)
      setBranch(repo.default_branch)
      fetchBranches(repo.owner.login, repo.name)
    }
  }

  const handleAnalyze = async () => {
    if (!selectedRepo) return
    setAnalyzing(true)

    // Store selection in session/URL and navigate to step 2
    const params = new URLSearchParams({
      owner: selectedRepo.owner.login,
      repo: selectedRepo.name,
      branch: branch
    })

    router.push(`/projects/import/analyze?${params}`)
  }

  return (
    <div className="container max-w-4xl py-8">
      <h1 className="text-2xl font-bold mb-6">Import Project from GitHub</h1>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <GitBranch className="h-5 w-5" />
            Step 1: Select Repository
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {loading ? (
            <div className="flex items-center gap-2">
              <Loader2 className="h-4 w-4 animate-spin" />
              Loading repositories...
            </div>
          ) : (
            <>
              <div className="space-y-2">
                <label className="text-sm font-medium">Repository</label>
                <Select onValueChange={handleRepoSelect}>
                  <SelectTrigger>
                    <SelectValue placeholder="Select a repository" />
                  </SelectTrigger>
                  <SelectContent>
                    {repos.map(repo => (
                      <SelectItem key={repo.id} value={repo.full_name}>
                        {repo.full_name}
                        {repo.private && <span className="ml-2 text-xs text-muted-foreground">Private</span>}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              {selectedRepo && (
                <div className="space-y-2">
                  <label className="text-sm font-medium">Branch</label>
                  <Select value={branch} onValueChange={setBranch}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {branches.map(b => (
                        <SelectItem key={b} value={b}>{b}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              )}

              <Button
                onClick={handleAnalyze}
                disabled={!selectedRepo || analyzing}
                className="w-full"
              >
                {analyzing ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    Analyzing Repository...
                  </>
                ) : (
                  <>
                    <FolderSearch className="mr-2 h-4 w-4" />
                    Analyze & Detect Services
                  </>
                )}
              </Button>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
```

---

## Task 2.4: Import Wizard - Step 2 (Service Detection & Configuration)

**File to Create**: `apps/switchyard-ui/app/(protected)/projects/import/analyze/page.tsx`

**Reference**: `docs/design/MONOREPO_PROJECT_MODEL.md` lines 477-535

```tsx
'use client'

import { useState, useEffect } from 'react'
import { useSearchParams, useRouter } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Loader2, Package, ArrowRight, AlertCircle, CheckCircle2 } from 'lucide-react'

interface DetectedService {
  name: string
  app_path: string
  runtime: string
  framework: string
  port: number
  build_command: string
  start_command: string
  confidence: number
  detection_notes: string[]
  selected?: boolean
}

interface AnalysisResult {
  monorepo_detected: boolean
  monorepo_tool: string
  services: DetectedService[]
  shared_paths: string[]
}

export default function AnalyzeResultsPage() {
  const searchParams = useSearchParams()
  const router = useRouter()

  const owner = searchParams.get('owner')!
  const repo = searchParams.get('repo')!
  const branch = searchParams.get('branch')!

  const [analysis, setAnalysis] = useState<AnalysisResult | null>(null)
  const [services, setServices] = useState<DetectedService[]>([])
  const [projectName, setProjectName] = useState(repo)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    analyzeRepository()
  }, [owner, repo, branch])

  const analyzeRepository = async () => {
    try {
      const res = await fetch(
        `/api/integrations/github/repos/${owner}/${repo}/analyze?branch=${branch}`
      )

      if (!res.ok) throw new Error('Failed to analyze repository')

      const data: AnalysisResult = await res.json()
      setAnalysis(data)
      setServices(data.services.map(s => ({ ...s, selected: true })))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Analysis failed')
    } finally {
      setLoading(false)
    }
  }

  const toggleService = (index: number) => {
    setServices(prev => prev.map((s, i) =>
      i === index ? { ...s, selected: !s.selected } : s
    ))
  }

  const updateService = (index: number, field: keyof DetectedService, value: any) => {
    setServices(prev => prev.map((s, i) =>
      i === index ? { ...s, [field]: value } : s
    ))
  }

  const handleContinue = () => {
    const selectedServices = services.filter(s => s.selected)

    // Store in session storage for next step
    sessionStorage.setItem('import_data', JSON.stringify({
      owner,
      repo,
      branch,
      projectName,
      services: selectedServices,
      monorepoTool: analysis?.monorepo_tool
    }))

    router.push('/projects/import/configure')
  }

  if (loading) {
    return (
      <div className="container max-w-4xl py-8 flex items-center justify-center">
        <div className="text-center">
          <Loader2 className="h-8 w-8 animate-spin mx-auto mb-4" />
          <p>Analyzing repository structure...</p>
          <p className="text-sm text-muted-foreground mt-2">
            Scanning for Dockerfiles, package.json, and other service indicators
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className="container max-w-4xl py-8">
      <h1 className="text-2xl font-bold mb-2">Configure Services</h1>
      <p className="text-muted-foreground mb-6">
        We detected {services.length} services in {owner}/{repo}
      </p>

      {analysis?.monorepo_detected && (
        <div className="bg-blue-50 border border-blue-200 rounded-lg p-4 mb-6 flex items-start gap-3">
          <Package className="h-5 w-5 text-blue-600 mt-0.5" />
          <div>
            <p className="font-medium text-blue-900">Monorepo Detected</p>
            <p className="text-sm text-blue-700">
              Using {analysis.monorepo_tool} • Shared paths: {analysis.shared_paths.join(', ')}
            </p>
          </div>
        </div>
      )}

      <div className="space-y-2 mb-6">
        <label className="text-sm font-medium">Project Name</label>
        <Input
          value={projectName}
          onChange={(e) => setProjectName(e.target.value)}
          placeholder="my-project"
        />
      </div>

      <div className="space-y-4 mb-6">
        <h2 className="text-lg font-semibold">Detected Services</h2>

        {services.map((service, index) => (
          <Card key={index} className={service.selected ? '' : 'opacity-50'}>
            <CardHeader className="pb-2">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <Checkbox
                    checked={service.selected}
                    onCheckedChange={() => toggleService(index)}
                  />
                  <div>
                    <CardTitle className="text-base">{service.name}</CardTitle>
                    <CardDescription>{service.app_path}</CardDescription>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <Badge variant="secondary">{service.runtime}</Badge>
                  {service.framework && (
                    <Badge variant="outline">{service.framework}</Badge>
                  )}
                  <Badge
                    variant={service.confidence > 0.8 ? 'default' : 'secondary'}
                  >
                    {Math.round(service.confidence * 100)}% confidence
                  </Badge>
                </div>
              </div>
            </CardHeader>

            {service.selected && (
              <CardContent className="pt-0">
                <div className="grid grid-cols-2 gap-4 mt-4">
                  <div>
                    <label className="text-sm font-medium">Port</label>
                    <Input
                      type="number"
                      value={service.port}
                      onChange={(e) => updateService(index, 'port', parseInt(e.target.value))}
                    />
                  </div>
                  <div>
                    <label className="text-sm font-medium">Build Command</label>
                    <Input
                      value={service.build_command}
                      onChange={(e) => updateService(index, 'build_command', e.target.value)}
                    />
                  </div>
                  <div className="col-span-2">
                    <label className="text-sm font-medium">Start Command</label>
                    <Input
                      value={service.start_command}
                      onChange={(e) => updateService(index, 'start_command', e.target.value)}
                    />
                  </div>
                </div>

                {service.detection_notes.length > 0 && (
                  <div className="mt-3 text-sm text-muted-foreground">
                    <p className="font-medium">Detection notes:</p>
                    <ul className="list-disc list-inside">
                      {service.detection_notes.map((note, i) => (
                        <li key={i}>{note}</li>
                      ))}
                    </ul>
                  </div>
                )}
              </CardContent>
            )}
          </Card>
        ))}
      </div>

      <div className="flex justify-between">
        <Button variant="outline" onClick={() => router.back()}>
          Back
        </Button>
        <Button
          onClick={handleContinue}
          disabled={!services.some(s => s.selected)}
        >
          Continue to Environment Setup
          <ArrowRight className="ml-2 h-4 w-4" />
        </Button>
      </div>
    </div>
  )
}
```

---

## Task 2.5: Import Wizard - Step 3 (Environment & Domain Setup)

**File to Create**: `apps/switchyard-ui/app/(protected)/projects/import/configure/page.tsx`

```tsx
'use client'

import { useState, useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Loader2, Globe, Lock, Rocket } from 'lucide-react'

interface ImportData {
  owner: string
  repo: string
  branch: string
  projectName: string
  services: any[]
  monorepoTool: string
}

interface EnvironmentConfig {
  name: string
  domain: string
  autoDeployBranch: string
  enableZeroTrust: boolean
}

export default function ConfigureEnvironmentPage() {
  const router = useRouter()
  const [importData, setImportData] = useState<ImportData | null>(null)
  const [environments, setEnvironments] = useState<EnvironmentConfig[]>([
    { name: 'production', domain: '', autoDeployBranch: 'main', enableZeroTrust: false },
    { name: 'staging', domain: '', autoDeployBranch: 'develop', enableZeroTrust: true },
  ])
  const [creating, setCreating] = useState(false)

  useEffect(() => {
    const data = sessionStorage.getItem('import_data')
    if (data) {
      const parsed = JSON.parse(data)
      setImportData(parsed)

      // Set default domains based on project name
      setEnvironments(prev => prev.map(env => ({
        ...env,
        domain: `${parsed.projectName}${env.name === 'production' ? '' : '-' + env.name}.enclii.app`
      })))
    }
  }, [])

  const updateEnvironment = (index: number, field: keyof EnvironmentConfig, value: any) => {
    setEnvironments(prev => prev.map((e, i) =>
      i === index ? { ...e, [field]: value } : e
    ))
  }

  const handleCreate = async () => {
    if (!importData) return
    setCreating(true)

    try {
      // 1. Create project
      const projectRes = await fetch('/api/v1/projects', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: importData.projectName,
          slug: importData.projectName.toLowerCase().replace(/\s+/g, '-'),
          description: `Imported from ${importData.owner}/${importData.repo}`,
          git_repo: `https://github.com/${importData.owner}/${importData.repo}`,
        })
      })

      const project = await projectRes.json()

      // 2. Create environments
      for (const env of environments) {
        await fetch('/api/v1/environments', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            project_id: project.id,
            name: env.name,
            kube_namespace: `${importData.projectName}-${env.name}`,
          })
        })
      }

      // 3. Create services
      for (const service of importData.services) {
        await fetch('/api/v1/services', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            project_id: project.id,
            name: service.name,
            git_repo: `https://github.com/${importData.owner}/${importData.repo}`,
            app_path: service.app_path,
            build_config: {
              runtime: service.runtime,
              build_command: service.build_command,
              start_command: service.start_command,
            },
          })
        })
      }

      // 4. Navigate to project page
      router.push(`/projects/${project.slug}`)

    } catch (error) {
      console.error('Failed to create project:', error)
    } finally {
      setCreating(false)
    }
  }

  if (!importData) {
    return <div>Loading...</div>
  }

  return (
    <div className="container max-w-4xl py-8">
      <h1 className="text-2xl font-bold mb-2">Environment Setup</h1>
      <p className="text-muted-foreground mb-6">
        Configure environments and domains for {importData.projectName}
      </p>

      <div className="space-y-4 mb-6">
        {environments.map((env, index) => (
          <Card key={env.name}>
            <CardHeader>
              <CardTitle className="text-base capitalize flex items-center gap-2">
                {env.name === 'production' ? (
                  <Rocket className="h-4 w-4 text-green-600" />
                ) : (
                  <Globe className="h-4 w-4 text-blue-600" />
                )}
                {env.name} Environment
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm font-medium">Domain</label>
                  <Input
                    value={env.domain}
                    onChange={(e) => updateEnvironment(index, 'domain', e.target.value)}
                    placeholder="app.example.com"
                  />
                </div>
                <div>
                  <label className="text-sm font-medium">Auto-deploy Branch</label>
                  <Input
                    value={env.autoDeployBranch}
                    onChange={(e) => updateEnvironment(index, 'autoDeployBranch', e.target.value)}
                    placeholder="main"
                  />
                </div>
              </div>

              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <Lock className="h-4 w-4 text-muted-foreground" />
                  <span className="text-sm">Require authentication (Zero Trust)</span>
                </div>
                <Switch
                  checked={env.enableZeroTrust}
                  onCheckedChange={(checked) => updateEnvironment(index, 'enableZeroTrust', checked)}
                />
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      <Card className="mb-6">
        <CardHeader>
          <CardTitle className="text-base">Summary</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <span className="text-muted-foreground">Repository:</span>
              <p className="font-mono">{importData.owner}/{importData.repo}</p>
            </div>
            <div>
              <span className="text-muted-foreground">Services:</span>
              <p>{importData.services.length} services detected</p>
            </div>
            <div>
              <span className="text-muted-foreground">Environments:</span>
              <p>{environments.length} environments</p>
            </div>
            <div>
              <span className="text-muted-foreground">Monorepo:</span>
              <p>{importData.monorepoTool || 'No'}</p>
            </div>
          </div>
        </CardContent>
      </Card>

      <div className="flex justify-between">
        <Button variant="outline" onClick={() => router.back()}>
          Back
        </Button>
        <Button onClick={handleCreate} disabled={creating}>
          {creating ? (
            <>
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              Creating Project...
            </>
          ) : (
            <>
              <Rocket className="mr-2 h-4 w-4" />
              Create Project & Deploy
            </>
          )}
        </Button>
      </div>
    </div>
  )
}
```

---

# PHASE 3: Cloudflare Tunnel & Domain Integration
**Estimated Time**: 2-3 weeks | **Priority**: 🟡 HIGH

## Task 3.1: Cloudflare Database Tables

**File to Create**: `apps/switchyard-api/internal/db/migrations/011_cloudflare_tunnels.up.sql`

**Reference**: `docs/design/CLOUDFLARE_TUNNEL_UI.md` lines 43-91

```sql
-- Cloudflare account configuration (platform level)
CREATE TABLE cloudflare_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id VARCHAR(255) NOT NULL UNIQUE,
    api_token_encrypted TEXT NOT NULL, -- encrypted with platform key
    zone_id VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Tunnels are scoped to environments
CREATE TABLE cloudflare_tunnels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    tunnel_id VARCHAR(255) NOT NULL, -- Cloudflare tunnel ID
    tunnel_name VARCHAR(255) NOT NULL,
    tunnel_token_encrypted TEXT NOT NULL,
    status VARCHAR(50) DEFAULT 'active', -- 'active', 'degraded', 'down'
    last_health_check TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT unique_tunnel_per_env UNIQUE(environment_id, tunnel_id)
);

-- Ingress rules for routing
CREATE TABLE tunnel_ingress_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tunnel_id UUID NOT NULL REFERENCES cloudflare_tunnels(id) ON DELETE CASCADE,
    hostname VARCHAR(255) NOT NULL,
    service_id UUID REFERENCES services(id) ON DELETE SET NULL,
    path VARCHAR(255) DEFAULT '/*',
    origin_port INTEGER NOT NULL,
    priority INTEGER DEFAULT 100,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT unique_hostname_path UNIQUE(tunnel_id, hostname, path)
);

-- Extend custom_domains table
ALTER TABLE custom_domains ADD COLUMN IF NOT EXISTS tunnel_rule_id UUID REFERENCES tunnel_ingress_rules(id);
ALTER TABLE custom_domains ADD COLUMN IF NOT EXISTS zero_trust_enabled BOOLEAN DEFAULT false;
ALTER TABLE custom_domains ADD COLUMN IF NOT EXISTS access_policy_id VARCHAR(255);

-- Indexes
CREATE INDEX idx_tunnels_environment ON cloudflare_tunnels(environment_id);
CREATE INDEX idx_ingress_hostname ON tunnel_ingress_rules(hostname);
CREATE INDEX idx_domains_tunnel ON custom_domains(tunnel_rule_id);

-- Triggers
CREATE TRIGGER update_cloudflare_accounts_updated_at
    BEFORE UPDATE ON cloudflare_accounts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_cloudflare_tunnels_updated_at
    BEFORE UPDATE ON cloudflare_tunnels
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

---

## Task 3.2: Cloudflare API Client

**File to Create**: `apps/switchyard-api/internal/cloudflare/client.go`

```go
package cloudflare

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
)

type Client struct {
    apiToken   string
    accountID  string
    httpClient *http.Client
    baseURL    string
}

func NewClient(apiToken, accountID string) *Client {
    return &Client{
        apiToken:   apiToken,
        accountID:  accountID,
        httpClient: &http.Client{},
        baseURL:    "https://api.cloudflare.com/client/v4",
    }
}

// Tunnel operations
func (c *Client) CreateTunnel(ctx context.Context, name string) (*Tunnel, error) {
    // POST /accounts/{account_id}/cfd_tunnel
}

func (c *Client) DeleteTunnel(ctx context.Context, tunnelID string) error {
    // DELETE /accounts/{account_id}/cfd_tunnel/{tunnel_id}
}

func (c *Client) GetTunnelToken(ctx context.Context, tunnelID string) (string, error) {
    // GET /accounts/{account_id}/cfd_tunnel/{tunnel_id}/token
}

func (c *Client) GetTunnelHealth(ctx context.Context, tunnelID string) (*TunnelHealth, error) {
    // GET /accounts/{account_id}/cfd_tunnel/{tunnel_id}/connections
}

// DNS operations
func (c *Client) CreateDNSRecord(ctx context.Context, zoneID string, record DNSRecord) error {
    // POST /zones/{zone_id}/dns_records
}

func (c *Client) DeleteDNSRecord(ctx context.Context, zoneID, recordID string) error {
    // DELETE /zones/{zone_id}/dns_records/{record_id}
}

func (c *Client) VerifyDNS(ctx context.Context, hostname string) (bool, error) {
    // Check if DNS resolves correctly
}

// Zero Trust Access operations
func (c *Client) CreateAccessPolicy(ctx context.Context, appID string, policy AccessPolicy) error {
    // POST /accounts/{account_id}/access/apps/{app_id}/policies
}

func (c *Client) UpdateAccessPolicy(ctx context.Context, appID, policyID string, policy AccessPolicy) error {
    // PUT /accounts/{account_id}/access/apps/{app_id}/policies/{policy_id}
}

// Types
type Tunnel struct {
    ID     string `json:"id"`
    Name   string `json:"name"`
    Status string `json:"status"`
}

type TunnelHealth struct {
    Connections []TunnelConnection `json:"connections"`
}

type TunnelConnection struct {
    ID          string `json:"id"`
    IsActive    bool   `json:"is_active"`
    Colo        string `json:"colo_name"`
    Origin      string `json:"origin_ip"`
}

type DNSRecord struct {
    Type    string `json:"type"`    // "CNAME", "A", "TXT"
    Name    string `json:"name"`    // hostname
    Content string `json:"content"` // target
    TTL     int    `json:"ttl"`
    Proxied bool   `json:"proxied"`
}

type AccessPolicy struct {
    Name     string        `json:"name"`
    Decision string        `json:"decision"` // "allow", "deny"
    Include  []PolicyRule  `json:"include"`
    Require  []PolicyRule  `json:"require,omitempty"`
}

type PolicyRule struct {
    Email   *EmailRule   `json:"email,omitempty"`
    Domain  *DomainRule  `json:"domain,omitempty"`
    Group   *GroupRule   `json:"group,omitempty"`
}

type EmailRule struct {
    Email string `json:"email"`
}

type DomainRule struct {
    Domain string `json:"domain"`
}

type GroupRule struct {
    GroupID string `json:"id"`
}
```

---

## Task 3.3: Domain Reconciler

**File to Create**: `apps/switchyard-api/internal/reconcilers/domain_reconciler.go`

**Reference**: `docs/design/CLOUDFLARE_TUNNEL_UI.md` lines 380-412

```go
package reconcilers

import (
    "context"
    "time"
)

// DomainReconciler ensures domain configurations match desired state
type DomainReconciler struct {
    cfClient      *cloudflare.Client
    domainRepo    db.CustomDomainRepository
    tunnelRepo    db.CloudflareTunnelRepository
    ingressRepo   db.TunnelIngressRepository
}

// ReconcileDomain ensures a domain is properly configured
// Flow: Verify DNS -> Create Ingress Rule -> Configure TLS -> Apply Zero Trust
func (r *DomainReconciler) ReconcileDomain(ctx context.Context, domainID uuid.UUID) error {
    domain, err := r.domainRepo.GetByID(ctx, domainID)
    if err != nil {
        return err
    }

    // Step 1: Verify DNS
    if !domain.Verified {
        verified, err := r.cfClient.VerifyDNS(ctx, domain.Domain)
        if err != nil {
            return err
        }
        if verified {
            domain.Verified = true
            r.domainRepo.Update(ctx, domain)
        } else {
            return fmt.Errorf("DNS not verified for %s", domain.Domain)
        }
    }

    // Step 2: Create/Update ingress rule
    if domain.TunnelRuleID == nil {
        tunnel, err := r.tunnelRepo.GetByEnvironment(ctx, domain.EnvironmentID)
        if err != nil {
            return err
        }

        rule := &TunnelIngressRule{
            TunnelID:   tunnel.ID,
            Hostname:   domain.Domain,
            ServiceID:  domain.ServiceID,
            OriginPort: domain.Port,
        }

        if err := r.ingressRepo.Create(ctx, rule); err != nil {
            return err
        }

        domain.TunnelRuleID = &rule.ID
        r.domainRepo.Update(ctx, domain)
    }

    // Step 3: Configure TLS (Cloudflare handles this automatically with proxy)
    if !domain.TLSEnabled {
        domain.TLSEnabled = true
        domain.TLSIssuer = "cloudflare"
        r.domainRepo.Update(ctx, domain)
    }

    // Step 4: Apply Zero Trust if enabled
    if domain.ZeroTrustEnabled && domain.AccessPolicyID == "" {
        policy := cloudflare.AccessPolicy{
            Name:     fmt.Sprintf("protect-%s", domain.Domain),
            Decision: "allow",
            Include: []cloudflare.PolicyRule{
                {Email: &cloudflare.EmailRule{Email: "*@example.com"}}, // Replace with tenant config
            },
        }

        policyID, err := r.cfClient.CreateAccessPolicy(ctx, domain.Domain, policy)
        if err != nil {
            return err
        }

        domain.AccessPolicyID = policyID
        r.domainRepo.Update(ctx, domain)
    }

    return nil
}

// StartReconcileLoop runs the reconciler on a schedule
func (r *DomainReconciler) StartReconcileLoop(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            r.reconcileAllPending(ctx)
        }
    }
}
```

---

## Task 3.4: Domain Management UI

**File to Create**: `apps/switchyard-ui/components/networking/domain-manager.tsx`

**Reference**: `docs/design/CLOUDFLARE_TUNNEL_UI.md` lines 239-362

```tsx
'use client'

import { useState, useEffect } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Globe, Lock, Plus, CheckCircle, AlertCircle, Clock, ExternalLink, Copy } from 'lucide-react'

interface Domain {
  id: string
  domain: string
  verified: boolean
  tls_enabled: boolean
  tls_issuer: string
  zero_trust_enabled: boolean
  service_id: string
  service_name: string
  created_at: string
}

interface DNSVerification {
  record_type: string
  record_name: string
  record_value: string
  verified: boolean
}

export function DomainManager({ serviceId, environmentId }: { serviceId: string; environmentId: string }) {
  const [domains, setDomains] = useState<Domain[]>([])
  const [loading, setLoading] = useState(true)
  const [showAddDialog, setShowAddDialog] = useState(false)
  const [newDomain, setNewDomain] = useState('')
  const [verification, setVerification] = useState<DNSVerification | null>(null)

  useEffect(() => {
    fetchDomains()
  }, [serviceId, environmentId])

  const fetchDomains = async () => {
    const res = await fetch(`/api/v1/services/${serviceId}/domains?environment_id=${environmentId}`)
    const data = await res.json()
    setDomains(data.domains || [])
    setLoading(false)
  }

  const addDomain = async () => {
    const res = await fetch(`/api/v1/services/${serviceId}/domains`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        domain: newDomain,
        environment_id: environmentId,
      })
    })

    const data = await res.json()

    if (data.verification_required) {
      setVerification(data.dns_verification)
    } else {
      setShowAddDialog(false)
      fetchDomains()
    }
  }

  const toggleZeroTrust = async (domainId: string, enabled: boolean) => {
    await fetch(`/api/v1/domains/${domainId}/zero-trust`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled })
    })
    fetchDomains()
  }

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-semibold">Custom Domains</h3>
        <Dialog open={showAddDialog} onOpenChange={setShowAddDialog}>
          <DialogTrigger asChild>
            <Button size="sm">
              <Plus className="h-4 w-4 mr-2" />
              Add Domain
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Add Custom Domain</DialogTitle>
            </DialogHeader>

            {!verification ? (
              <div className="space-y-4">
                <div>
                  <label className="text-sm font-medium">Domain</label>
                  <Input
                    value={newDomain}
                    onChange={(e) => setNewDomain(e.target.value)}
                    placeholder="app.example.com"
                  />
                </div>
                <Button onClick={addDomain} className="w-full">
                  Add Domain
                </Button>
              </div>
            ) : (
              <div className="space-y-4">
                <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-4">
                  <p className="font-medium text-yellow-900 mb-2">DNS Verification Required</p>
                  <p className="text-sm text-yellow-700 mb-4">
                    Add the following DNS record to verify domain ownership:
                  </p>

                  <div className="bg-white rounded border p-3 space-y-2">
                    <div className="flex justify-between">
                      <span className="text-sm text-muted-foreground">Type:</span>
                      <span className="font-mono text-sm">{verification.record_type}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-sm text-muted-foreground">Name:</span>
                      <div className="flex items-center gap-2">
                        <span className="font-mono text-sm">{verification.record_name}</span>
                        <Button variant="ghost" size="sm" onClick={() => copyToClipboard(verification.record_name)}>
                          <Copy className="h-3 w-3" />
                        </Button>
                      </div>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-sm text-muted-foreground">Value:</span>
                      <div className="flex items-center gap-2">
                        <span className="font-mono text-sm truncate max-w-[200px]">{verification.record_value}</span>
                        <Button variant="ghost" size="sm" onClick={() => copyToClipboard(verification.record_value)}>
                          <Copy className="h-3 w-3" />
                        </Button>
                      </div>
                    </div>
                  </div>
                </div>

                <Button onClick={fetchDomains} className="w-full">
                  I've Added the DNS Record
                </Button>
              </div>
            )}
          </DialogContent>
        </Dialog>
      </div>

      {domains.length === 0 ? (
        <Card>
          <CardContent className="py-8 text-center text-muted-foreground">
            <Globe className="h-8 w-8 mx-auto mb-2 opacity-50" />
            <p>No custom domains configured</p>
            <p className="text-sm">Add a domain to make this service accessible via a custom URL</p>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-2">
          {domains.map(domain => (
            <Card key={domain.id}>
              <CardContent className="py-4">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <Globe className="h-5 w-5 text-muted-foreground" />
                    <div>
                      <div className="flex items-center gap-2">
                        <span className="font-medium">{domain.domain}</span>
                        {domain.verified ? (
                          <Badge variant="default" className="gap-1">
                            <CheckCircle className="h-3 w-3" />
                            Verified
                          </Badge>
                        ) : (
                          <Badge variant="secondary" className="gap-1">
                            <Clock className="h-3 w-3" />
                            Pending Verification
                          </Badge>
                        )}
                        {domain.tls_enabled && (
                          <Badge variant="outline" className="gap-1">
                            <Lock className="h-3 w-3" />
                            TLS
                          </Badge>
                        )}
                      </div>
                      <p className="text-sm text-muted-foreground">
                        Routes to {domain.service_name}
                      </p>
                    </div>
                  </div>

                  <div className="flex items-center gap-4">
                    <div className="flex items-center gap-2">
                      <Lock className="h-4 w-4 text-muted-foreground" />
                      <span className="text-sm">Zero Trust</span>
                      <Switch
                        checked={domain.zero_trust_enabled}
                        onCheckedChange={(checked) => toggleZeroTrust(domain.id, checked)}
                      />
                    </div>

                    <Button variant="ghost" size="sm" asChild>
                      <a href={`https://${domain.domain}`} target="_blank" rel="noopener noreferrer">
                        <ExternalLink className="h-4 w-4" />
                      </a>
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}
```

---

# PHASE 4: Webhook & Auto-Deploy
**Estimated Time**: 1-2 weeks | **Priority**: 🟡 HIGH

## Task 4.1: Enhanced Webhook Handler

**File to Modify**: `apps/switchyard-api/internal/api/webhook_handlers.go`

**Reference**: `docs/design/MONOREPO_PROJECT_MODEL.md` lines 614-649

```go
// Enhanced GitHub webhook handler with affected service detection
func (h *Handler) HandleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
    // 1. Verify webhook signature
    payload, err := github.ValidatePayload(r, []byte(h.webhookSecret))
    if err != nil {
        http.Error(w, "Invalid signature", http.StatusUnauthorized)
        return
    }

    event, err := github.ParseWebHook(github.WebHookType(r), payload)
    if err != nil {
        http.Error(w, "Invalid payload", http.StatusBadRequest)
        return
    }

    switch e := event.(type) {
    case *github.PushEvent:
        h.handlePushEvent(r.Context(), e)
    case *github.PullRequestEvent:
        h.handlePREvent(r.Context(), e)
    }

    w.WriteHeader(http.StatusOK)
}

func (h *Handler) handlePushEvent(ctx context.Context, event *github.PushEvent) {
    repoURL := event.GetRepo().GetHTMLURL()
    branch := strings.TrimPrefix(event.GetRef(), "refs/heads/")
    commitSHA := event.GetAfter()

    // 1. Find all services using this repo
    services, err := h.serviceRepo.FindByGitRepo(ctx, repoURL)
    if err != nil || len(services) == 0 {
        return
    }

    // 2. For monorepo: determine affected services
    affectedServices := h.detectAffectedServices(ctx, services, event)

    // 3. Find environments configured to auto-deploy this branch
    for _, svc := range affectedServices {
        envs, _ := h.envRepo.FindByAutoDeploy(ctx, svc.ProjectID, branch)

        for _, env := range envs {
            // 4. Trigger deployment
            h.deploymentGroupSvc.CreateGroupDeployment(
                ctx,
                svc.ProjectID,
                env.ID,
                []uuid.UUID{svc.ID}, // Just the affected services
                "webhook",
                commitSHA,
                "", // No PR for direct push
            )
        }
    }
}

// detectAffectedServices analyzes commits to find which services changed
func (h *Handler) detectAffectedServices(
    ctx context.Context,
    services []*Service,
    event *github.PushEvent,
) []*Service {
    // Get changed files from commits
    changedFiles := make(map[string]bool)
    for _, commit := range event.Commits {
        for _, file := range commit.Added {
            changedFiles[file] = true
        }
        for _, file := range commit.Modified {
            changedFiles[file] = true
        }
        for _, file := range commit.Removed {
            changedFiles[file] = true
        }
    }

    // Match changed files to services based on app_path
    affected := make([]*Service, 0)
    for _, svc := range services {
        if svc.AppPath == "" || svc.AppPath == "." {
            // Root-level service, always affected
            affected = append(affected, svc)
            continue
        }

        for file := range changedFiles {
            if strings.HasPrefix(file, svc.AppPath+"/") {
                affected = append(affected, svc)
                break
            }
        }
    }

    // Also check shared paths (packages/, libs/)
    sharedPaths := []string{"packages/", "libs/", "shared/"}
    sharedChanged := false
    for file := range changedFiles {
        for _, shared := range sharedPaths {
            if strings.HasPrefix(file, shared) {
                sharedChanged = true
                break
            }
        }
    }

    // If shared code changed, rebuild all services that depend on it
    if sharedChanged {
        // For now, rebuild all services in monorepo
        // Future: track which services depend on which shared packages
        return services
    }

    return affected
}
```

---

## Task 4.2: Auto-Deploy Configuration

**Add to environments table**: `apps/switchyard-api/internal/db/migrations/012_auto_deploy.up.sql`

```sql
ALTER TABLE environments ADD COLUMN IF NOT EXISTS auto_deploy_branch VARCHAR(255);
ALTER TABLE environments ADD COLUMN IF NOT EXISTS auto_deploy_enabled BOOLEAN DEFAULT false;
ALTER TABLE environments ADD COLUMN IF NOT EXISTS require_approval BOOLEAN DEFAULT false;
ALTER TABLE environments ADD COLUMN IF NOT EXISTS approval_team_id UUID REFERENCES teams(id);

CREATE INDEX idx_environments_auto_deploy ON environments(project_id, auto_deploy_branch)
    WHERE auto_deploy_enabled = true;
```

---

# PHASE 5: Team Management & UI Polish
**Estimated Time**: 1-2 weeks | **Priority**: 🟢 MEDIUM

## Task 5.1: Team Management API

**File to Create**: `apps/switchyard-api/internal/api/team_handlers.go`

```go
// POST /v1/teams - Create team
// GET /v1/teams - List teams
// GET /v1/teams/{id} - Get team
// PUT /v1/teams/{id} - Update team
// DELETE /v1/teams/{id} - Delete team
// POST /v1/teams/{id}/members - Add member
// DELETE /v1/teams/{id}/members/{user_id} - Remove member
// GET /v1/teams/{id}/members - List members
```

## Task 5.2: Team Management UI

**Files to Create**:
- `apps/switchyard-ui/app/(protected)/teams/page.tsx`
- `apps/switchyard-ui/app/(protected)/teams/[id]/page.tsx`
- `apps/switchyard-ui/components/teams/team-members.tsx`
- `apps/switchyard-ui/components/teams/invite-member.tsx`

## Task 5.3: API Keys Management

**Files to Create**:
- `apps/switchyard-api/internal/api/apikey_handlers.go`
- `apps/switchyard-ui/app/(protected)/settings/api-keys/page.tsx`

---

# PHASE 6: Monitoring & Observability
**Estimated Time**: 1 week | **Priority**: 🟢 MEDIUM

## Task 6.1: Deployment Metrics Dashboard

**File to Create**: `apps/switchyard-ui/app/(protected)/monitoring/page.tsx`

Features:
- Deployment success rate
- Build duration trends
- Service health status
- Recent deployments timeline

## Task 6.2: Service Logs UI Enhancement

**File to Modify**: `apps/switchyard-ui/app/(protected)/services/[id]/logs/page.tsx`

Features:
- Real-time log streaming (WebSocket)
- Log level filtering
- Search within logs
- Download logs

---

# Testing & Quality Assurance

## Integration Tests Required

```go
// apps/switchyard-api/internal/api/integration_test.go

func TestMultiServiceDeployment(t *testing.T) {
    // 1. Create project with 3 services (api, web, worker)
    // 2. Set dependencies (web -> api, worker -> api)
    // 3. Trigger group deployment
    // 4. Verify deployment order: api first, then (web, worker) in parallel
    // 5. Verify all services are running
}

func TestWebhookAffectedServiceDetection(t *testing.T) {
    // 1. Create monorepo project with 3 services
    // 2. Simulate webhook with changes only in apps/api/
    // 3. Verify only api service is redeployed
}

func TestDomainVerificationFlow(t *testing.T) {
    // 1. Add custom domain
    // 2. Verify DNS verification record is returned
    // 3. Mock DNS verification
    // 4. Verify TLS is enabled
    // 5. Toggle Zero Trust and verify policy is created
}

func TestRollbackGroup(t *testing.T) {
    // 1. Deploy 3 services successfully
    // 2. Trigger another deployment where one service fails
    // 3. Trigger group rollback
    // 4. Verify all services rolled back to previous version
}
```

## E2E Tests

```typescript
// apps/switchyard-ui/e2e/import-project.spec.ts

test('import monorepo project', async ({ page }) => {
  // Navigate to import wizard
  await page.goto('/projects/import')

  // Select repository
  await page.getByRole('combobox').click()
  await page.getByRole('option', { name: 'madfam-io/dhanam' }).click()

  // Analyze
  await page.getByRole('button', { name: 'Analyze' }).click()

  // Verify services detected
  await expect(page.getByText('api')).toBeVisible()
  await expect(page.getByText('web')).toBeVisible()

  // Configure and create
  await page.getByRole('button', { name: 'Continue' }).click()
  await page.getByRole('button', { name: 'Create Project' }).click()

  // Verify project created
  await expect(page).toHaveURL(/\/projects\/dhanam/)
})
```

---

# Success Criteria

## MVP Feature Checklist

### Core Features (Must Have)
- [ ] Multi-service deployment groups with dependency ordering
- [ ] Topological sort for deployment order
- [ ] Group-level rollback
- [ ] Repository analysis endpoint
- [ ] Service auto-detection (Dockerfile, package.json, requirements.txt, go.mod)
- [ ] Monorepo import wizard (3 steps)
- [ ] Custom domain management
- [ ] DNS verification flow
- [ ] Zero Trust toggle

### Secondary Features (Should Have)
- [ ] Webhook auto-deploy with affected service detection
- [ ] Team management UI
- [ ] API keys management
- [ ] Deployment metrics dashboard

### Polish (Nice to Have)
- [ ] Real-time log streaming
- [ ] Service dependency graph visualization
- [ ] Canary deployment UI

---

# Execution Order

**Critical Path** (blocks everything):
1. Task 1.1-1.6: Deployment Groups → enables multi-service deploy
2. Task 2.1-2.5: Service Detection → enables monorepo import

**Parallel Track A** (can start after Phase 1):
- Task 3.1-3.4: Cloudflare Integration

**Parallel Track B** (can start after Phase 2):
- Task 4.1-4.2: Webhook Auto-Deploy

**Final Polish** (after core features):
- Task 5.1-5.3: Team Management
- Task 6.1-6.2: Monitoring Dashboard

---

# Estimated Timeline

| Phase | Duration | Dependencies |
|-------|----------|--------------|
| Phase 1: Deployment Groups | 2-3 weeks | None |
| Phase 2: Service Detection | 2 weeks | Phase 1 (can overlap) |
| Phase 3: Cloudflare | 2 weeks | Phase 1 complete |
| Phase 4: Webhooks | 1-2 weeks | Phase 1+2 complete |
| Phase 5: Teams | 1 week | Any time |
| Phase 6: Monitoring | 1 week | Any time |

**Total: 8-10 weeks** for full MVP parity

---

# Reference Documents

- **Monorepo Design**: `docs/design/MONOREPO_PROJECT_MODEL.md`
- **Cloudflare Design**: `docs/design/CLOUDFLARE_TUNNEL_UI.md`
- **Existing API Code**: `apps/switchyard-api/internal/api/`
- **Existing UI Code**: `apps/switchyard-ui/app/(protected)/`
- **Database Schema**: `apps/switchyard-api/internal/db/migrations/`

---

*This prompt provides complete implementation specifications for achieving Vercel/Railway feature parity. Each task includes specific file paths, code templates, and references to existing design documents.*
