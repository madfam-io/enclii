/**
 * Pipeline Types
 * Type definitions for the unified pipeline progress component
 */

import type { LucideIcon } from "lucide-react";

// ============================================================================
// Component Props
// ============================================================================

export interface UnifiedPipelineProgressProps {
  serviceId: string;
  commitSha: string;
  serviceName?: string;
  onComplete?: () => void;
  onError?: (error: string) => void;
  pollInterval?: number;
  className?: string;
}

// ============================================================================
// Pipeline Status Types
// ============================================================================

export type StepStatus = "pending" | "active" | "completed" | "failed" | "skipped";

export interface PipelineStep {
  id: string;
  label: string;
  icon: React.ReactNode;
  description?: string;
}

// ============================================================================
// Sub-component Props
// ============================================================================

export interface StepIconProps {
  status: StepStatus;
}

export interface WorkflowRunItemProps {
  run: import("@/lib/types/pipeline").CIRun;
}

export interface StageDetailsProps {
  status: import("@/lib/types/pipeline").UnifiedBuildStatus;
  stepId: string;
  expanded: boolean;
  onToggle: () => void;
}
