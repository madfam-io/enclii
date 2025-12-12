// Shared TypeScript interfaces for Enclii UI

// ============================================================================
// GitHub Integration Types
// ============================================================================

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

export interface BuildConfig {
  type: "buildpack" | "dockerfile";
  port: number;
  dockerfile_path?: string;
  build_args?: Record<string, string>;
}

// ============================================================================
// Release Types
// ============================================================================

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
