/**
 * Types for the Unified Pipeline Progress component
 *
 * These types correspond to the UnifiedBuildStatus API response from
 * GET /v1/services/:id/builds/:commit_sha/status
 */

// ============================================================================
// CI Run Types (GitHub Actions)
// ============================================================================

export type CIRunStatus = "queued" | "in_progress" | "completed";
export type CIRunConclusion =
  | "success"
  | "failure"
  | "cancelled"
  | "skipped"
  | "timed_out"
  | "action_required";

export interface CIRun {
  id: string;
  service_id: string;
  commit_sha: string;
  workflow_name: string;
  workflow_id: number;
  run_id: number;
  run_number: number;
  status: CIRunStatus;
  conclusion?: CIRunConclusion;
  html_url: string;
  branch?: string;
  event_type?: string;
  actor?: string;
  started_at?: string;
  completed_at?: string;
  created_at: string;
  updated_at: string;
}

// ============================================================================
// GitHub Actions Status
// ============================================================================

export interface GitHubActionsStatus {
  workflows: CIRun[];
  overall_status: "pending" | "in_progress" | "success" | "failure";
  total_runs: number;
  success_count: number;
  failure_count: number;
  in_progress: number;
}

// ============================================================================
// Roundhouse Build Status
// ============================================================================

export interface RoundhouseRelease {
  id: string;
  version: string;
  status: "pending" | "building" | "ready" | "failed";
  git_sha?: string;
  image_uri?: string;
  sbom?: string;
  image_signature?: string;
  error_message?: string;
  created_at: string;
  completed_at?: string;
}

export interface RoundhouseStatus {
  release?: RoundhouseRelease;
  status: "building" | "ready" | "failed";
  image_uri?: string;
  error_message?: string;
  has_sbom: boolean;
  has_signature: boolean;
}

// ============================================================================
// Deployment Status
// ============================================================================

export interface DeploymentInfo {
  id: string;
  status: "pending" | "running" | "failed";
  health: string;
  replicas?: number;
  ready_replicas?: number;
  error_message?: string;
  created_at: string;
  updated_at?: string;
}

export interface DeploymentProgressStatus {
  deployment?: DeploymentInfo;
  status: "pending" | "running" | "failed";
  health: string;
  error_message?: string;
}

// ============================================================================
// Build Stage (for UI rendering)
// ============================================================================

export interface BuildStage {
  name: string;
  status: "pending" | "in_progress" | "success" | "failure" | "skipped";
  started_at?: string;
  completed_at?: string;
  error_message?: string;
  url?: string;
}

// ============================================================================
// Unified Build Status (main response type)
// ============================================================================

export type OverallPipelineStatus =
  | "pending"
  | "building"
  | "ready"
  | "deploying"
  | "running"
  | "failed";

export interface UnifiedBuildStatus {
  commit_sha: string;
  service_id: string;
  service_name: string;
  github_actions?: GitHubActionsStatus;
  roundhouse?: RoundhouseStatus;
  deployment?: DeploymentProgressStatus;
  overall_status: OverallPipelineStatus;
  stages: BuildStage[];
}

// ============================================================================
// Cross-service commit status response
// ============================================================================

export interface CommitBuildStatusResponse {
  commit_sha: string;
  services: UnifiedBuildStatus[];
}
