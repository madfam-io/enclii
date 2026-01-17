/**
 * Pipeline Utilities
 * Helper functions for pipeline status processing and display
 */

import type { StepStatus } from "./pipeline-types";
import type {
  UnifiedBuildStatus,
  BuildStage,
  OverallPipelineStatus,
} from "@/lib/types/pipeline";

// ============================================================================
// Time Formatting
// ============================================================================

/**
 * Format duration between two timestamps in human-readable form
 * @param startedAt - ISO timestamp string for start time
 * @param endedAt - ISO timestamp string for end time (defaults to now)
 * @returns Formatted duration string (e.g., "45s", "2m 30s", "1h 15m")
 */
export function formatDuration(startedAt: string, endedAt?: string): string {
  const start = new Date(startedAt);
  const end = endedAt ? new Date(endedAt) : new Date();
  const diffMs = end.getTime() - start.getTime();
  const diffSec = Math.floor(diffMs / 1000);

  if (diffSec < 60) return `${diffSec}s`;
  const minutes = Math.floor(diffSec / 60);
  const seconds = diffSec % 60;
  if (minutes < 60) return `${minutes}m ${seconds}s`;
  const hours = Math.floor(minutes / 60);
  const mins = minutes % 60;
  return `${hours}h ${mins}m`;
}

// ============================================================================
// Status Mapping
// ============================================================================

/**
 * Get status indicator class for overall pipeline status
 * Uses semantic status colors for theme compatibility
 */
export function getOverallStatusColor(status: OverallPipelineStatus): string {
  switch (status) {
    case "running":
      return "bg-status-success";
    case "ready":
      return "bg-status-info";
    case "building":
    case "deploying":
      return "bg-status-warning";
    case "failed":
      return "bg-status-error";
    default:
      return "bg-status-neutral";
  }
}

/**
 * Get human-readable label for overall pipeline status
 */
export function getOverallStatusLabel(status: OverallPipelineStatus): string {
  switch (status) {
    case "running":
      return "Running";
    case "ready":
      return "Ready to Deploy";
    case "building":
      return "Building";
    case "deploying":
      return "Deploying";
    case "failed":
      return "Failed";
    default:
      return "Pending";
  }
}

/**
 * Map build stage status to step status
 */
export function mapStageStatus(stage: BuildStage): StepStatus {
  switch (stage.status) {
    case "success":
      return "completed";
    case "failure":
      return "failed";
    case "in_progress":
      return "active";
    case "skipped":
      return "skipped";
    default:
      return "pending";
  }
}

/**
 * Determine step status from unified build status
 */
export function getStepStatusFromBuildStatus(
  stepId: string,
  status: UnifiedBuildStatus | null
): StepStatus {
  if (!status) return "pending";

  switch (stepId) {
    case "ci":
      if (!status.github_actions) return "pending";
      switch (status.github_actions.overall_status) {
        case "success":
          return "completed";
        case "failure":
          return "failed";
        case "in_progress":
          return "active";
        default:
          return "pending";
      }
    case "build":
      if (!status.roundhouse) return "pending";
      switch (status.roundhouse.status) {
        case "ready":
          return "completed";
        case "failed":
          return "failed";
        case "building":
          return "active";
        default:
          return "pending";
      }
    case "deploy":
      if (!status.deployment) return "pending";
      switch (status.deployment.status) {
        case "running":
          return "completed";
        case "failed":
          return "failed";
        case "pending":
          return "active";
        default:
          return "pending";
      }
    default:
      return "pending";
  }
}
