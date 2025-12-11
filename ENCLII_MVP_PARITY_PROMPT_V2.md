# Enclii MVP Feature Parity Implementation Prompt v2.0
## Complete Vercel/Railway Competitive Feature Implementation Guide

> **Updated**: Based on current implementation status as of session end
> **Purpose**: Comprehensive, step-by-step prompt for a Software Engineering agent to implement all remaining features needed for Vercel/Railway MVP feature parity.

---

## Executive Summary - UPDATED

| Area | Status | Completion |
|------|--------|------------|
| **Core API (Switchyard)** | 🟢 Mostly Complete | ~85% |
| **Monorepo Detection & Import** | 🟢 Implemented | ~90% |
| **Deployment Groups** | 🟢 Database & Handlers | ~80% |
| **Environment Variables** | 🟢 Complete | 100% |
| **Service Settings** | 🟢 Complete | 100% |
| **Custom Domains** | 🟡 Partial | ~60% |
| **Preview Environments** | 🔴 Not Started | 0% |
| **Team Management** | 🔴 Not Started | 0% |
| **Real-time Logs** | 🔴 Not Started | 0% |
| **Metrics Dashboard** | 🔴 Not Started | 0% |

**Overall MVP Completion**: ~70%
**Remaining Work**: ~120 hours

---

## What's Already Implemented (DO NOT REBUILD)

### ✅ Completed Features

#### Backend (Switchyard API)
- **Authentication**: JWT + OIDC via Janua (`internal/auth/`)
- **Projects CRUD**: Full project management (`projects_handlers.go`)
- **Services CRUD**: Create, Read, Update, Delete (`service_handlers.go`, `services_handlers.go`)
- **Deployments**: Single and group deployments (`deployment_handlers.go`, `deployment_group_handlers.go`)
- **Deployment Groups**: Database schema + repository + handlers (migration `010_deployment_groups.up.sql`)
- **Environment Variables**: Full CRUD with encryption (`envvar_handlers.go`, `envvar_repository.go`)
- **Custom Domains**: Basic CRUD (`domain_handlers.go`, `custom_domain_repository.go`)
- **Routes**: Ingress configuration (`route_repository.go`, `networking_handlers.go`)
- **GitHub Integration**: OAuth linking, repo listing, branch listing, repository analysis (`integrations_handlers.go`)
- **Monorepo Detection**: Service detection from Dockerfile, package.json, go.mod, requirements.txt
- **Auto-Deploy Config**: Database schema (migration `006_auto_deploy.up.sql`)
- **Build Configuration**: Builder detection, Nixpacks/Docker support
- **Webhooks**: GitHub webhook handling (`webhook_handlers.go`)

#### Frontend (Switchyard UI)
- **Authentication Flow**: Login, logout, session management (`contexts/AuthContext.tsx`)
- **Dashboard**: Overview page (`app/(protected)/page.tsx`)
- **Projects List**: Project listing and creation
- **Services List**: Service listing (`app/(protected)/services/page.tsx`)
- **Service Detail**: Tabs for Deployments, Logs, Env Vars, Networking, Settings (`app/(protected)/services/[id]/page.tsx`)
- **Import Wizard Step 1**: GitHub repo selection (`app/(protected)/services/import/page.tsx`)
- **Import Wizard Step 2**: Service detection & selection (`app/(protected)/services/import/[owner]/[repo]/page.tsx`)
- **Environment Variables Tab**: Full CRUD with masked values (`components/env-vars/EnvVarsTab.tsx`)
- **Networking Tab**: Custom domain management (`components/networking/NetworkingTab.tsx`)
- **Settings Tab**: Service update/delete (`components/settings/SettingsTab.tsx`)
- **API Client**: With CSRF protection, auth headers (`lib/api.ts`)

#### Database Migrations (Already Applied)
- `000_consolidated_schema.up.sql` - Core tables
- `001_initial_schema.up.sql` - Base entities
- `002_compliance_schema.up.sql` - Audit tables
- `003_rotation_audit_logs.up.sql` - Log rotation
- `004_custom_domains_routes.up.sql` - Networking
- `005_oidc_support.up.sql` - OIDC integration
- `006_auto_deploy.up.sql` - Auto-deploy configuration
- `007_github_integration.up.sql` - GitHub OAuth
- `008_monorepo_support.up.sql` - Monorepo fields
- `009_cloudflare_tunnel.up.sql` - Tunnel support
- `010_deployment_groups.up.sql` - Deployment groups & service dependencies
- `011_environment_variables.up.sql` - Env vars table

---

# REMAINING WORK - IMPLEMENTATION TASKS

## PHASE 1: Preview Environments (PR-based Deployments)
**Priority**: 🔴 CRITICAL | **Estimated Time**: 2 weeks

### Why Critical
Vercel and Railway's killer feature is automatic preview deployments for every PR. Without this, Enclii isn't competitive.

### Task 1.1: Preview Environment Database Schema

**File to Create**: `apps/switchyard-api/internal/db/migrations/012_preview_environments.up.sql`

```sql
-- Preview environments are auto-created for PRs
CREATE TABLE preview_environments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,

    -- PR metadata
    pr_number INTEGER NOT NULL,
    pr_title TEXT,
    pr_url TEXT NOT NULL,
    pr_author VARCHAR(255),
    pr_branch VARCHAR(255) NOT NULL,
    pr_base_branch VARCHAR(255) NOT NULL DEFAULT 'main',
    commit_sha VARCHAR(40),

    -- Preview URL
    preview_subdomain VARCHAR(255) NOT NULL UNIQUE,
    preview_url TEXT GENERATED ALWAYS AS ('https://' || preview_subdomain || '.preview.enclii.app') STORED,

    -- Lifecycle
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    -- 'pending', 'building', 'deploying', 'active', 'sleeping', 'failed', 'deleted'

    -- Resource management
    auto_sleep_after INTERVAL DEFAULT '24 hours',
    last_accessed_at TIMESTAMPTZ,
    sleeping_since TIMESTAMPTZ,

    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT unique_pr_per_project UNIQUE(project_id, pr_number)
);

-- Preview deployments link to regular deployments
ALTER TABLE deployments ADD COLUMN preview_environment_id UUID REFERENCES preview_environments(id);

-- Track preview services (subset of project services deployed to preview)
CREATE TABLE preview_services (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    preview_environment_id UUID NOT NULL REFERENCES preview_environments(id) ON DELETE CASCADE,
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    deployment_id UUID REFERENCES deployments(id),
    preview_url TEXT,
    status VARCHAR(50) DEFAULT 'pending',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_preview_envs_project ON preview_environments(project_id);
CREATE INDEX idx_preview_envs_status ON preview_environments(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_preview_envs_subdomain ON preview_environments(preview_subdomain);
CREATE INDEX idx_preview_services_preview ON preview_services(preview_environment_id);
CREATE INDEX idx_deployments_preview ON deployments(preview_environment_id) WHERE preview_environment_id IS NOT NULL;

-- Trigger
CREATE TRIGGER update_preview_environments_updated_at
    BEFORE UPDATE ON preview_environments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

**Down Migration**: `012_preview_environments.down.sql`
```sql
DROP TABLE IF EXISTS preview_services;
ALTER TABLE deployments DROP COLUMN IF EXISTS preview_environment_id;
DROP TABLE IF EXISTS preview_environments;
```

### Task 1.2: Preview Environment Repository

**File to Create**: `apps/switchyard-api/internal/db/preview_repository.go`

```go
package db

import (
    "context"
    "database/sql"
    "fmt"
    "strings"
    "time"

    "github.com/google/uuid"
)

type PreviewEnvironment struct {
    ID               uuid.UUID      `db:"id"`
    ProjectID        uuid.UUID      `db:"project_id"`
    EnvironmentID    uuid.UUID      `db:"environment_id"`
    PRNumber         int            `db:"pr_number"`
    PRTitle          sql.NullString `db:"pr_title"`
    PRURL            string         `db:"pr_url"`
    PRAuthor         sql.NullString `db:"pr_author"`
    PRBranch         string         `db:"pr_branch"`
    PRBaseBranch     string         `db:"pr_base_branch"`
    CommitSHA        sql.NullString `db:"commit_sha"`
    PreviewSubdomain string         `db:"preview_subdomain"`
    PreviewURL       string         `db:"preview_url"`
    Status           string         `db:"status"`
    AutoSleepAfter   time.Duration  `db:"auto_sleep_after"`
    LastAccessedAt   sql.NullTime   `db:"last_accessed_at"`
    SleepingSince    sql.NullTime   `db:"sleeping_since"`
    CreatedAt        time.Time      `db:"created_at"`
    UpdatedAt        time.Time      `db:"updated_at"`
    DeletedAt        sql.NullTime   `db:"deleted_at"`
}

type PreviewService struct {
    ID                   uuid.UUID      `db:"id"`
    PreviewEnvironmentID uuid.UUID      `db:"preview_environment_id"`
    ServiceID            uuid.UUID      `db:"service_id"`
    DeploymentID         uuid.NullUUID  `db:"deployment_id"`
    PreviewURL           sql.NullString `db:"preview_url"`
    Status               string         `db:"status"`
    CreatedAt            time.Time      `db:"created_at"`
}

type PreviewRepositoryInterface interface {
    Create(ctx context.Context, preview *PreviewEnvironment) error
    GetByID(ctx context.Context, id uuid.UUID) (*PreviewEnvironment, error)
    GetByPRNumber(ctx context.Context, projectID uuid.UUID, prNumber int) (*PreviewEnvironment, error)
    GetBySubdomain(ctx context.Context, subdomain string) (*PreviewEnvironment, error)
    ListByProject(ctx context.Context, projectID uuid.UUID, includeDeleted bool) ([]*PreviewEnvironment, error)
    ListActive(ctx context.Context) ([]*PreviewEnvironment, error)
    UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
    UpdateCommitSHA(ctx context.Context, id uuid.UUID, sha string) error
    MarkAccessed(ctx context.Context, id uuid.UUID) error
    MarkSleeping(ctx context.Context, id uuid.UUID) error
    WakeUp(ctx context.Context, id uuid.UUID) error
    SoftDelete(ctx context.Context, id uuid.UUID) error

    // Preview services
    AddPreviewService(ctx context.Context, svc *PreviewService) error
    GetPreviewServices(ctx context.Context, previewID uuid.UUID) ([]*PreviewService, error)
    UpdatePreviewServiceStatus(ctx context.Context, id uuid.UUID, status string, deploymentID *uuid.UUID) error
}

type PreviewRepository struct {
    db *sql.DB
}

func NewPreviewRepository(db *sql.DB) *PreviewRepository {
    return &PreviewRepository{db: db}
}

// GenerateSubdomain creates a unique preview subdomain
// Format: pr-{number}-{project-slug}-{random}
func GeneratePreviewSubdomain(projectSlug string, prNumber int) string {
    randomSuffix := uuid.New().String()[:8]
    slug := strings.ToLower(strings.ReplaceAll(projectSlug, " ", "-"))
    if len(slug) > 20 {
        slug = slug[:20]
    }
    return fmt.Sprintf("pr-%d-%s-%s", prNumber, slug, randomSuffix)
}

func (r *PreviewRepository) Create(ctx context.Context, preview *PreviewEnvironment) error {
    query := `
        INSERT INTO preview_environments (
            id, project_id, environment_id, pr_number, pr_title, pr_url,
            pr_author, pr_branch, pr_base_branch, commit_sha, preview_subdomain,
            status, auto_sleep_after
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
        RETURNING created_at, updated_at, preview_url
    `

    if preview.ID == uuid.Nil {
        preview.ID = uuid.New()
    }

    return r.db.QueryRowContext(ctx, query,
        preview.ID, preview.ProjectID, preview.EnvironmentID,
        preview.PRNumber, preview.PRTitle, preview.PRURL,
        preview.PRAuthor, preview.PRBranch, preview.PRBaseBranch,
        preview.CommitSHA, preview.PreviewSubdomain,
        preview.Status, preview.AutoSleepAfter,
    ).Scan(&preview.CreatedAt, &preview.UpdatedAt, &preview.PreviewURL)
}

// Implement remaining methods following existing repository patterns...
```

### Task 1.3: Preview Environment Handlers

**File to Create**: `apps/switchyard-api/internal/api/preview_handlers.go`

```go
package api

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
)

// CreatePreviewEnvironment creates a new preview for a PR
// POST /v1/projects/:slug/previews
func (h *Handler) CreatePreviewEnvironment(c *gin.Context) {
    var req struct {
        PRNumber     int      `json:"pr_number" binding:"required"`
        PRTitle      string   `json:"pr_title"`
        PRURL        string   `json:"pr_url" binding:"required"`
        PRAuthor     string   `json:"pr_author"`
        PRBranch     string   `json:"pr_branch" binding:"required"`
        PRBaseBranch string   `json:"pr_base_branch"`
        CommitSHA    string   `json:"commit_sha"`
        ServiceIDs   []string `json:"service_ids"` // Optional: specific services to deploy
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Implementation:
    // 1. Get project by slug
    // 2. Get or create preview environment for this PR
    // 3. Generate unique subdomain
    // 4. Create preview environment record
    // 5. Trigger deployments for all/selected services
    // 6. Return preview URL
}

// ListPreviewEnvironments lists all previews for a project
// GET /v1/projects/:slug/previews
func (h *Handler) ListPreviewEnvironments(c *gin.Context) {
    // List active preview environments with their services and status
}

// GetPreviewEnvironment gets a specific preview
// GET /v1/projects/:slug/previews/:id
func (h *Handler) GetPreviewEnvironment(c *gin.Context) {}

// UpdatePreviewEnvironment updates preview (e.g., new commit)
// PATCH /v1/projects/:slug/previews/:id
func (h *Handler) UpdatePreviewEnvironment(c *gin.Context) {
    // Used when new commits are pushed to the PR
    // Triggers rebuild of affected services
}

// DeletePreviewEnvironment deletes a preview (when PR is closed/merged)
// DELETE /v1/projects/:slug/previews/:id
func (h *Handler) DeletePreviewEnvironment(c *gin.Context) {}

// WakePreviewEnvironment wakes a sleeping preview
// POST /v1/projects/:slug/previews/:id/wake
func (h *Handler) WakePreviewEnvironment(c *gin.Context) {}
```

### Task 1.4: GitHub PR Webhook Handler Enhancement

**File to Modify**: `apps/switchyard-api/internal/api/webhook_handlers.go`

Add PR event handling:

```go
func (h *Handler) handlePullRequestEvent(ctx context.Context, event *github.PullRequestEvent) {
    action := event.GetAction()
    pr := event.GetPullRequest()
    repo := event.GetRepo()

    repoURL := repo.GetHTMLURL()

    // Find project by repo URL
    project, err := h.repos.Projects.FindByGitRepo(ctx, repoURL)
    if err != nil || project == nil {
        return // Not a tracked repository
    }

    switch action {
    case "opened", "reopened", "synchronize":
        // Create or update preview environment
        h.createOrUpdatePreview(ctx, project, pr)

    case "closed":
        // Delete preview environment
        h.deletePreview(ctx, project, pr.GetNumber())
    }
}

func (h *Handler) createOrUpdatePreview(ctx context.Context, project *Project, pr *github.PullRequest) {
    // Check if preview already exists
    existing, err := h.repos.Previews.GetByPRNumber(ctx, project.ID, pr.GetNumber())

    if existing != nil {
        // Update existing - trigger rebuild
        h.repos.Previews.UpdateCommitSHA(ctx, existing.ID, pr.GetHead().GetSHA())
        h.previewSvc.RebuildPreview(ctx, existing.ID)
    } else {
        // Create new preview environment
        subdomain := db.GeneratePreviewSubdomain(project.Slug, pr.GetNumber())

        preview := &db.PreviewEnvironment{
            ProjectID:        project.ID,
            EnvironmentID:    project.DefaultPreviewEnvID, // Need to add this to projects
            PRNumber:         pr.GetNumber(),
            PRTitle:          sql.NullString{String: pr.GetTitle(), Valid: true},
            PRURL:            pr.GetHTMLURL(),
            PRAuthor:         sql.NullString{String: pr.GetUser().GetLogin(), Valid: true},
            PRBranch:         pr.GetHead().GetRef(),
            PRBaseBranch:     pr.GetBase().GetRef(),
            CommitSHA:        sql.NullString{String: pr.GetHead().GetSHA(), Valid: true},
            PreviewSubdomain: subdomain,
            Status:           "pending",
        }

        h.repos.Previews.Create(ctx, preview)
        h.previewSvc.DeployPreview(ctx, preview)
    }
}
```

### Task 1.5: Preview Environment UI

**File to Create**: `apps/switchyard-ui/app/(protected)/projects/[slug]/previews/page.tsx`

```tsx
'use client'

import { useState, useEffect, use } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { apiGet, apiPost, apiDelete } from '@/lib/api'
import { ExternalLink, GitPullRequest, Clock, Trash2, Play } from 'lucide-react'

interface PreviewEnvironment {
  id: string
  pr_number: number
  pr_title: string
  pr_url: string
  pr_author: string
  pr_branch: string
  commit_sha: string
  preview_url: string
  status: 'pending' | 'building' | 'deploying' | 'active' | 'sleeping' | 'failed'
  created_at: string
  last_accessed_at: string | null
  services: PreviewService[]
}

interface PreviewService {
  id: string
  service_name: string
  status: string
  preview_url: string
}

interface PageProps {
  params: Promise<{ slug: string }>
}

export default function PreviewsPage({ params }: PageProps) {
  const { slug } = use(params)
  const [previews, setPreviews] = useState<PreviewEnvironment[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetchPreviews()
  }, [slug])

  const fetchPreviews = async () => {
    try {
      const data = await apiGet<{ previews: PreviewEnvironment[] }>(
        `/v1/projects/${slug}/previews`
      )
      setPreviews(data.previews || [])
    } catch (err) {
      console.error('Failed to fetch previews:', err)
    } finally {
      setLoading(false)
    }
  }

  const wakePreview = async (previewId: string) => {
    await apiPost(`/v1/projects/${slug}/previews/${previewId}/wake`, {})
    fetchPreviews()
  }

  const deletePreview = async (previewId: string) => {
    if (!confirm('Are you sure you want to delete this preview?')) return
    await apiDelete(`/v1/projects/${slug}/previews/${previewId}`)
    fetchPreviews()
  }

  const statusColors = {
    pending: 'bg-yellow-100 text-yellow-800',
    building: 'bg-blue-100 text-blue-800',
    deploying: 'bg-purple-100 text-purple-800',
    active: 'bg-green-100 text-green-800',
    sleeping: 'bg-gray-100 text-gray-800',
    failed: 'bg-red-100 text-red-800',
  }

  return (
    <div className="container mx-auto py-8">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold">Preview Environments</h1>
          <p className="text-muted-foreground">
            Automatic deployments for pull requests
          </p>
        </div>
      </div>

      {loading ? (
        <div className="grid gap-4">
          {[1, 2, 3].map(i => (
            <Card key={i} className="animate-pulse">
              <CardContent className="h-24" />
            </Card>
          ))}
        </div>
      ) : previews.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center">
            <GitPullRequest className="h-12 w-12 mx-auto mb-4 text-gray-400" />
            <h3 className="text-lg font-medium mb-2">No Preview Environments</h3>
            <p className="text-muted-foreground">
              Open a pull request on GitHub to automatically create a preview deployment.
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-4">
          {previews.map(preview => (
            <Card key={preview.id}>
              <CardHeader className="pb-2">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <GitPullRequest className="h-5 w-5 text-purple-600" />
                    <div>
                      <CardTitle className="text-base flex items-center gap-2">
                        #{preview.pr_number}: {preview.pr_title}
                        <Badge className={statusColors[preview.status]}>
                          {preview.status}
                        </Badge>
                      </CardTitle>
                      <p className="text-sm text-muted-foreground">
                        {preview.pr_branch} • by {preview.pr_author}
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    {preview.status === 'sleeping' && (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => wakePreview(preview.id)}
                      >
                        <Play className="h-4 w-4 mr-1" />
                        Wake
                      </Button>
                    )}
                    {preview.status === 'active' && (
                      <Button size="sm" variant="outline" asChild>
                        <a
                          href={preview.preview_url}
                          target="_blank"
                          rel="noopener noreferrer"
                        >
                          <ExternalLink className="h-4 w-4 mr-1" />
                          Open Preview
                        </a>
                      </Button>
                    )}
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => deletePreview(preview.id)}
                    >
                      <Trash2 className="h-4 w-4 text-red-500" />
                    </Button>
                  </div>
                </div>
              </CardHeader>
              <CardContent>
                <div className="flex items-center gap-4 text-sm text-muted-foreground">
                  <span className="font-mono">{preview.commit_sha?.slice(0, 7)}</span>
                  <span className="flex items-center gap-1">
                    <Clock className="h-3 w-3" />
                    {new Date(preview.created_at).toLocaleDateString()}
                  </span>
                  {preview.preview_url && (
                    <span className="font-mono text-xs">{preview.preview_url}</span>
                  )}
                </div>

                {/* Service statuses */}
                {preview.services?.length > 0 && (
                  <div className="mt-3 flex gap-2">
                    {preview.services.map(svc => (
                      <Badge key={svc.id} variant="outline">
                        {svc.service_name}: {svc.status}
                      </Badge>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}
```

### Task 1.6: Preview Subdomain Routing (Cloudflare)

**File to Modify**: `apps/switchyard-api/internal/reconcilers/domain_reconciler.go`

Add wildcard subdomain support for `*.preview.enclii.app`:

```go
// ReconcilePreviewSubdomain ensures preview subdomain routes correctly
func (r *DomainReconciler) ReconcilePreviewSubdomain(ctx context.Context, preview *PreviewEnvironment) error {
    // 1. Create/update ingress rule for preview subdomain
    // 2. Point to the service's internal ClusterIP
    // 3. Update tunnel configuration if using Cloudflare Tunnel

    // For Cloudflare: Use wildcard DNS record *.preview.enclii.app -> tunnel
    // Tunnel config routes based on hostname
}
```

---

## PHASE 2: Team Management & RBAC
**Priority**: 🟡 HIGH | **Estimated Time**: 1.5 weeks

### Task 2.1: Teams Database Schema

**File to Create**: `apps/switchyard-api/internal/db/migrations/013_teams.up.sql`

```sql
-- Teams table
CREATE TABLE teams (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    avatar_url TEXT,
    owner_id UUID NOT NULL, -- References user from Janua
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Team memberships
CREATE TABLE team_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id UUID NOT NULL, -- References user from Janua
    role VARCHAR(50) NOT NULL DEFAULT 'member',
    -- 'owner', 'admin', 'developer', 'viewer'
    invited_by UUID,
    invited_at TIMESTAMPTZ,
    accepted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT unique_team_member UNIQUE(team_id, user_id)
);

-- Team invitations
CREATE TABLE team_invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'member',
    token VARCHAR(255) NOT NULL UNIQUE,
    invited_by UUID NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Link projects to teams
ALTER TABLE projects ADD COLUMN team_id UUID REFERENCES teams(id);
CREATE INDEX idx_projects_team ON projects(team_id);

-- Indexes
CREATE INDEX idx_team_members_team ON team_members(team_id);
CREATE INDEX idx_team_members_user ON team_members(user_id);
CREATE INDEX idx_team_invitations_team ON team_invitations(team_id);
CREATE INDEX idx_team_invitations_email ON team_invitations(email);
CREATE INDEX idx_team_invitations_token ON team_invitations(token);

-- Triggers
CREATE TRIGGER update_teams_updated_at
    BEFORE UPDATE ON teams
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

### Task 2.2: Team Handlers

**File to Create**: `apps/switchyard-api/internal/api/team_handlers.go`

```go
package api

// Team CRUD
// POST /v1/teams
// GET /v1/teams
// GET /v1/teams/:id
// PATCH /v1/teams/:id
// DELETE /v1/teams/:id

// Team Members
// GET /v1/teams/:id/members
// POST /v1/teams/:id/members (invite)
// PATCH /v1/teams/:id/members/:userId (change role)
// DELETE /v1/teams/:id/members/:userId (remove)

// Invitations
// POST /v1/teams/:id/invitations
// GET /v1/invitations/:token (public - for accepting)
// POST /v1/invitations/:token/accept
// DELETE /v1/teams/:id/invitations/:invitationId (cancel)
```

### Task 2.3: Teams UI

**Files to Create**:
- `apps/switchyard-ui/app/(protected)/teams/page.tsx` - List teams
- `apps/switchyard-ui/app/(protected)/teams/new/page.tsx` - Create team
- `apps/switchyard-ui/app/(protected)/teams/[id]/page.tsx` - Team detail
- `apps/switchyard-ui/app/(protected)/teams/[id]/members/page.tsx` - Member management
- `apps/switchyard-ui/app/(protected)/teams/[id]/settings/page.tsx` - Team settings
- `apps/switchyard-ui/components/teams/InviteMemberModal.tsx`
- `apps/switchyard-ui/components/teams/TeamMemberList.tsx`

---

## PHASE 3: Real-time Logs & Metrics
**Priority**: 🟡 HIGH | **Estimated Time**: 1 week

### Task 3.1: WebSocket Log Streaming

**File to Create**: `apps/switchyard-api/internal/api/logs_websocket.go`

```go
package api

import (
    "context"
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
    CheckOrigin: func(r *http.Request) bool {
        // Validate origin for security
        return true // TODO: Proper origin check
    },
}

// StreamServiceLogs streams real-time logs via WebSocket
// GET /v1/services/:id/logs/stream
func (h *Handler) StreamServiceLogs(c *gin.Context) {
    serviceID := c.Param("id")

    conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        h.logger.Error(c.Request.Context(), "WebSocket upgrade failed", logging.Error("error", err))
        return
    }
    defer conn.Close()

    // Get service details
    service, err := h.repos.Services.GetByID(uuid.MustParse(serviceID))
    if err != nil {
        conn.WriteJSON(gin.H{"error": "Service not found"})
        return
    }

    // Start streaming logs from Kubernetes
    ctx, cancel := context.WithCancel(c.Request.Context())
    defer cancel()

    logChan := h.k8sClient.StreamPodLogs(ctx, service.Namespace, service.PodSelector)

    // Read messages for close signal
    go func() {
        for {
            if _, _, err := conn.ReadMessage(); err != nil {
                cancel()
                return
            }
        }
    }()

    // Stream logs to client
    for logEntry := range logChan {
        if err := conn.WriteJSON(logEntry); err != nil {
            return
        }
    }
}
```

### Task 3.2: Logs UI with WebSocket

**File to Modify**: `apps/switchyard-ui/app/(protected)/services/[id]/page.tsx`

Add to logs tab:

```tsx
// components/logs/LogsViewer.tsx
'use client'

import { useState, useEffect, useRef } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Download, Search, Pause, Play, Filter } from 'lucide-react'

interface LogEntry {
  timestamp: string
  level: 'info' | 'warn' | 'error' | 'debug'
  message: string
  source: string
}

interface LogsViewerProps {
  serviceId: string
}

export function LogsViewer({ serviceId }: LogsViewerProps) {
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [connected, setConnected] = useState(false)
  const [paused, setPaused] = useState(false)
  const [filter, setFilter] = useState('')
  const [levelFilter, setLevelFilter] = useState<string>('all')
  const wsRef = useRef<WebSocket | null>(null)
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    connectWebSocket()
    return () => wsRef.current?.close()
  }, [serviceId])

  const connectWebSocket = () => {
    const wsUrl = `${process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:4200'}/v1/services/${serviceId}/logs/stream`
    const ws = new WebSocket(wsUrl)

    ws.onopen = () => setConnected(true)
    ws.onclose = () => setConnected(false)
    ws.onmessage = (event) => {
      if (paused) return
      const entry = JSON.parse(event.data)
      setLogs(prev => [...prev.slice(-1000), entry]) // Keep last 1000 entries

      // Auto-scroll if at bottom
      if (containerRef.current) {
        const { scrollTop, scrollHeight, clientHeight } = containerRef.current
        if (scrollHeight - scrollTop <= clientHeight + 100) {
          containerRef.current.scrollTop = scrollHeight
        }
      }
    }

    wsRef.current = ws
  }

  const downloadLogs = () => {
    const content = logs.map(l => `${l.timestamp} [${l.level}] ${l.message}`).join('\n')
    const blob = new Blob([content], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `logs-${serviceId}-${Date.now()}.txt`
    a.click()
  }

  const filteredLogs = logs.filter(log => {
    if (levelFilter !== 'all' && log.level !== levelFilter) return false
    if (filter && !log.message.toLowerCase().includes(filter.toLowerCase())) return false
    return true
  })

  const levelColors = {
    info: 'text-blue-500',
    warn: 'text-yellow-500',
    error: 'text-red-500',
    debug: 'text-gray-500',
  }

  return (
    <div className="h-full flex flex-col">
      {/* Controls */}
      <div className="flex items-center gap-4 p-4 border-b">
        <div className="flex items-center gap-2">
          <div className={`w-2 h-2 rounded-full ${connected ? 'bg-green-500' : 'bg-red-500'}`} />
          <span className="text-sm">{connected ? 'Connected' : 'Disconnected'}</span>
        </div>

        <div className="flex-1 flex items-center gap-2">
          <Search className="h-4 w-4 text-gray-400" />
          <Input
            placeholder="Search logs..."
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            className="max-w-xs"
          />
        </div>

        <Select value={levelFilter} onValueChange={setLevelFilter}>
          <option value="all">All Levels</option>
          <option value="error">Error</option>
          <option value="warn">Warning</option>
          <option value="info">Info</option>
          <option value="debug">Debug</option>
        </Select>

        <Button variant="outline" size="sm" onClick={() => setPaused(!paused)}>
          {paused ? <Play className="h-4 w-4" /> : <Pause className="h-4 w-4" />}
        </Button>

        <Button variant="outline" size="sm" onClick={downloadLogs}>
          <Download className="h-4 w-4" />
        </Button>
      </div>

      {/* Log entries */}
      <div
        ref={containerRef}
        className="flex-1 overflow-auto bg-gray-900 p-4 font-mono text-sm"
      >
        {filteredLogs.map((log, i) => (
          <div key={i} className="flex gap-2 hover:bg-gray-800 py-0.5">
            <span className="text-gray-500 shrink-0">
              {new Date(log.timestamp).toLocaleTimeString()}
            </span>
            <span className={`shrink-0 ${levelColors[log.level]}`}>
              [{log.level.toUpperCase()}]
            </span>
            <span className="text-gray-300">{log.message}</span>
          </div>
        ))}

        {filteredLogs.length === 0 && (
          <div className="text-gray-500 text-center py-8">
            {logs.length === 0 ? 'Waiting for logs...' : 'No logs match your filter'}
          </div>
        )}
      </div>
    </div>
  )
}
```

### Task 3.3: Metrics Dashboard

**File to Create**: `apps/switchyard-ui/app/(protected)/monitoring/page.tsx`

```tsx
'use client'

import { useState, useEffect } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { apiGet } from '@/lib/api'
import { Activity, Clock, CheckCircle, XCircle, Zap } from 'lucide-react'

interface DashboardMetrics {
  deployments: {
    total_today: number
    success_rate: number
    avg_duration_seconds: number
    recent: RecentDeployment[]
  }
  services: {
    total: number
    healthy: number
    unhealthy: number
  }
  builds: {
    in_progress: number
    queued: number
    avg_duration_seconds: number
  }
}

interface RecentDeployment {
  id: string
  service_name: string
  project_name: string
  status: string
  duration_seconds: number
  created_at: string
}

export default function MonitoringPage() {
  const [metrics, setMetrics] = useState<DashboardMetrics | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetchMetrics()
    const interval = setInterval(fetchMetrics, 30000) // Refresh every 30s
    return () => clearInterval(interval)
  }, [])

  const fetchMetrics = async () => {
    try {
      const data = await apiGet<DashboardMetrics>('/v1/monitoring/dashboard')
      setMetrics(data)
    } catch (err) {
      console.error('Failed to fetch metrics:', err)
    } finally {
      setLoading(false)
    }
  }

  if (loading || !metrics) {
    return <div>Loading...</div>
  }

  return (
    <div className="container mx-auto py-8">
      <h1 className="text-2xl font-bold mb-6">Platform Monitoring</h1>

      {/* Overview Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-8">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Deployments Today
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{metrics.deployments.total_today}</div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Success Rate
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-600">
              {(metrics.deployments.success_rate * 100).toFixed(1)}%
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Avg Deploy Time
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {Math.round(metrics.deployments.avg_duration_seconds)}s
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Service Health
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              <span className="text-green-600">{metrics.services.healthy}</span>
              <span className="text-muted-foreground mx-1">/</span>
              <span>{metrics.services.total}</span>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Recent Deployments */}
      <Card>
        <CardHeader>
          <CardTitle>Recent Deployments</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-2">
            {metrics.deployments.recent.map(deploy => (
              <div
                key={deploy.id}
                className="flex items-center justify-between py-2 border-b last:border-0"
              >
                <div className="flex items-center gap-3">
                  {deploy.status === 'succeeded' ? (
                    <CheckCircle className="h-5 w-5 text-green-500" />
                  ) : deploy.status === 'failed' ? (
                    <XCircle className="h-5 w-5 text-red-500" />
                  ) : (
                    <Activity className="h-5 w-5 text-blue-500 animate-pulse" />
                  )}
                  <div>
                    <p className="font-medium">{deploy.service_name}</p>
                    <p className="text-sm text-muted-foreground">{deploy.project_name}</p>
                  </div>
                </div>
                <div className="text-right">
                  <p className="text-sm">
                    <Clock className="h-3 w-3 inline mr-1" />
                    {deploy.duration_seconds}s
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {new Date(deploy.created_at).toLocaleTimeString()}
                  </p>
                </div>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
```

---

## PHASE 4: Missing UI Components & Polish
**Priority**: 🟢 MEDIUM | **Estimated Time**: 1 week

### Task 4.1: Missing shadcn/ui Components

The codebase uses some shadcn/ui components that may not be installed. Install:

```bash
cd apps/switchyard-ui
npx shadcn@latest add checkbox select dialog switch tabs badge
```

### Task 4.2: API Keys Management

**File to Create**: `apps/switchyard-ui/app/(protected)/settings/api-keys/page.tsx`

For CI/CD integration - users need API keys for CLI and pipelines.

### Task 4.3: Billing/Usage Dashboard

**File to Modify**: `apps/switchyard-ui/app/(protected)/billing/page.tsx`

Add resource usage tracking:
- Build minutes used
- Deployment count
- Storage used
- Bandwidth consumed

### Task 4.4: Service Dependency Graph Visualization

**File to Create**: `apps/switchyard-ui/components/services/DependencyGraph.tsx`

Interactive graph showing service dependencies using a library like react-flow or d3.

---

## PHASE 5: Testing & Quality
**Priority**: 🟢 MEDIUM | **Estimated Time**: 3 days

### Task 5.1: Integration Tests

**File to Create**: `apps/switchyard-api/internal/api/preview_test.go`

```go
func TestPreviewEnvironmentLifecycle(t *testing.T) {
    // 1. Create project with services
    // 2. Simulate PR webhook (opened)
    // 3. Verify preview environment created
    // 4. Verify preview services deployed
    // 5. Simulate PR webhook (synchronize - new commit)
    // 6. Verify rebuild triggered
    // 7. Simulate PR webhook (closed)
    // 8. Verify preview deleted
}

func TestPreviewAutoSleep(t *testing.T) {
    // 1. Create preview
    // 2. Wait for auto-sleep duration
    // 3. Verify preview status is 'sleeping'
    // 4. Access preview URL
    // 5. Verify preview wakes up
}
```

### Task 5.2: E2E Tests

**File to Create**: `apps/switchyard-ui/e2e/preview-environments.spec.ts`

```typescript
import { test, expect } from '@playwright/test'

test('view preview environments', async ({ page }) => {
  await page.goto('/projects/my-project/previews')
  await expect(page.getByText('Preview Environments')).toBeVisible()
})

test('wake sleeping preview', async ({ page }) => {
  // Test the wake functionality
})
```

---

## Success Metrics

### MVP Checklist

#### Core Features (Must Have for MVP)
- [x] Project/Service CRUD
- [x] Single-service deployment
- [x] GitHub integration (OAuth, repo listing)
- [x] Monorepo service detection
- [x] Multi-service import wizard
- [x] Environment variables management
- [x] Service settings (update/delete)
- [x] Deployment groups (database + handlers)
- [x] Custom domains (basic)
- [ ] **Preview environments (automatic PR deployments)** ← PRIORITY 1
- [ ] **Team management & invitations** ← PRIORITY 2
- [ ] **Real-time log streaming** ← PRIORITY 3

#### Nice to Have
- [ ] Service dependency graph visualization
- [ ] Canary deployment UI
- [ ] API keys management UI
- [ ] Detailed metrics dashboard
- [ ] Billing/usage tracking

---

## Execution Order

**Critical Path** (blocks user adoption):
1. Preview Environments (Tasks 1.1-1.6) - THE differentiating feature
2. Team Management (Tasks 2.1-2.3) - Required for multi-user

**Parallel Work**:
- Real-time Logs (Task 3.1-3.2) - Can develop alongside previews
- Metrics Dashboard (Task 3.3) - Independent

**Polish Phase**:
- Missing UI components (Task 4.1-4.4)
- Testing (Task 5.1-5.2)

---

## Estimated Timeline

| Phase | Duration | Dependencies |
|-------|----------|--------------|
| Phase 1: Preview Environments | 2 weeks | None |
| Phase 2: Team Management | 1.5 weeks | Can start after Phase 1 begins |
| Phase 3: Logs & Metrics | 1 week | Independent |
| Phase 4: UI Polish | 1 week | After core features |
| Phase 5: Testing | 3 days | After all features |

**Total: ~5-6 weeks** for complete MVP parity

---

## Reference Documents

- **Existing Implementation**: This prompt documents what's already built
- **Design Docs**: `docs/design/MONOREPO_PROJECT_MODEL.md`, `docs/design/CLOUDFLARE_TUNNEL_UI.md`
- **API Code**: `apps/switchyard-api/internal/api/`
- **UI Code**: `apps/switchyard-ui/app/(protected)/`
- **Database Migrations**: `apps/switchyard-api/internal/db/migrations/`

---

*This prompt v2.0 reflects the current implementation state and focuses only on remaining work. Each task includes specific file paths, code templates, and references to existing patterns in the codebase.*
