/**
 * Shared TypeScript interfaces for Enclii UI
 *
 * This module defines all TypeScript interfaces used across the Enclii
 * web dashboard. These types correspond to API responses from the
 * Switchyard API (api.enclii.dev).
 *
 * @module types
 */

// ============================================================================
// GitHub Integration Types
// ============================================================================

/**
 * Represents a GitHub repository from the GitHub API.
 * Used in repository selection flows for service creation.
 */
export interface GitHubRepository {
  id: number;
  name: string;
  full_name: string;
  private: boolean;
  default_branch: string;
  clone_url: string;
  html_url: string;
  description: string;
  language: string;
  updated_at: string;
  owner: {
    login: string;
    avatar_url: string;
  };
}

export interface GitHubReposResponse {
  repositories: GitHubRepository[];
  total_count: number;
}

export interface IntegrationStatus {
  provider: string;
  linked: boolean;
  provider_email?: string;
  can_access_repos: boolean;
  message?: string;
}

// ============================================================================
// Repository Analysis Types
// ============================================================================

export interface DetectedService {
  name: string;
  app_path: string;
  runtime: string;
  framework: string;
  port: number;
  build_command: string;
  start_command: string;
  confidence: number;
  detection_notes: string[];
  has_dockerfile: boolean;
  dependencies?: string[];
}

export interface AnalysisResult {
  monorepo_detected: boolean;
  monorepo_tool: string;
  services: DetectedService[];
  shared_paths: string[];
  analyzed_at: string;
  branch: string;
  commit_sha?: string;
}

export interface GitHubBranch {
  name: string;
  protected: boolean;
}

export interface BranchesResponse {
  branches: GitHubBranch[];
  count: number;
}

// ============================================================================
// Project Types
// ============================================================================

/**
 * Represents a project in the Enclii platform.
 *
 * Projects are the top-level organizational unit. Each project can contain
 * multiple services, environments, and team members.
 *
 * @example
 * const project: Project = {
 *   id: "550e8400-e29b-41d4-a716-446655440000",
 *   name: "My App",
 *   slug: "my-app"
 * };
 */
export interface Project {
  id: string;
  name: string;
  slug: string;
  description?: string;
  created_at?: string;
  updated_at?: string;
}

export interface ProjectsResponse {
  projects: Project[];
}

// ============================================================================
// Service Types
// ============================================================================

/**
 * Represents a deployable service in Enclii.
 *
 * Services are the primary deployable units. Each service is linked to
 * a git repository and can be deployed to multiple environments.
 *
 * @property id - Unique service identifier (UUID)
 * @property name - Human-readable service name
 * @property project_id - Parent project ID
 * @property git_repo - Source repository URL
 * @property git_branch - Default branch for deployments
 * @property build_config - Build configuration (buildpack or dockerfile)
 */
export interface Service {
  id: string;
  name: string;
  project_id: string;
  git_repo?: string;
  git_branch?: string;
  build_config?: BuildConfig;
  created_at?: string;
  updated_at?: string;
}

export interface ServiceOverview {
  id: string;
  name: string;
  project_name: string;
  project_slug?: string;
  environment: string;
  status: "healthy" | "unhealthy" | "unknown";
  version: string;
  replicas: string;
}

/**
 * Build configuration for a service.
 *
 * Enclii supports two build modes:
 * - `buildpack`: Auto-detected build using Cloud Native Buildpacks
 * - `dockerfile`: Custom Dockerfile-based builds
 *
 * @property type - Build type (buildpack or dockerfile)
 * @property port - Port the service listens on
 * @property dockerfile_path - Path to Dockerfile (for dockerfile type)
 * @property build_args - Additional build arguments
 */
export interface BuildConfig {
  type: "buildpack" | "dockerfile";
  port: number;
  dockerfile_path?: string;
  build_args?: Record<string, string>;
}

// ============================================================================
// Release Types
// ============================================================================

/**
 * Represents a release (build + deploy) of a service.
 *
 * Releases track the full lifecycle from code commit to deployment:
 * pending → building → ready → deploying → deployed
 *
 * @property id - Unique release identifier
 * @property version - Semantic version or auto-generated version
 * @property status - Current release status
 * @property git_sha - Git commit SHA this release was built from
 * @property image_tag - Container image tag
 * @property error_message - Error details if status is 'failed'
 */
export interface Release {
  id: string;
  version: string;
  status: "pending" | "building" | "ready" | "failed" | "deploying" | "deployed";
  git_sha?: string;
  image_tag?: string;
  created_at: string;
  completed_at?: string;
  error_message?: string;
}

export type BuildStage = "pending" | "running" | "completed" | "failed";

export interface BuildStep {
  name: string;
  status: BuildStage;
  duration?: string;
  message?: string;
}

// ============================================================================
// Dashboard Types
// ============================================================================

export interface DashboardStats {
  total_projects: number;
  total_services: number;
  healthy_services: number;
  unhealthy_services: number;
  total_deployments: number;
  recent_deployments: number;
}

export interface DashboardActivity {
  id: string;
  type: "deployment" | "service_created" | "project_created" | "build_started" | "build_completed";
  description: string;
  timestamp: string;
  service_id?: string;
  project_id?: string;
}

export interface DashboardResponse {
  stats: DashboardStats;
  activities: DashboardActivity[];
  services: ServiceOverview[];
}

// ============================================================================
// Environment Types
// ============================================================================

export interface Environment {
  id: string;
  name: string;
  slug: string;
  project_id: string;
  is_production: boolean;
  variables?: Record<string, string>;
  created_at?: string;
}

// ============================================================================
// API Response Types
// ============================================================================

export interface ApiError {
  error: string;
  message: string;
  status_code: number;
  details?: Record<string, unknown>;
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
}
