"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import {
  CheckCircle2,
  Circle,
  Loader2,
  XCircle,
  ExternalLink,
  GitBranch,
  GitCommit,
  Package,
  Server,
  PlayCircle,
  ChevronDown,
  ChevronRight,
  AlertCircle,
  Shield,
  FileText,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { apiGet } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type {
  UnifiedBuildStatus,
  BuildStage,
  OverallPipelineStatus,
  CIRun,
} from "@/lib/types/pipeline";

// ============================================================================
// Types
// ============================================================================

interface UnifiedPipelineProgressProps {
  serviceId: string;
  commitSha: string;
  serviceName?: string;
  onComplete?: () => void;
  onError?: (error: string) => void;
  pollInterval?: number;
  className?: string;
}

type StepStatus = "pending" | "active" | "completed" | "failed" | "skipped";

interface PipelineStep {
  id: string;
  label: string;
  icon: React.ReactNode;
  description?: string;
}

// ============================================================================
// Constants
// ============================================================================

const PIPELINE_STEPS: PipelineStep[] = [
  {
    id: "ci",
    label: "CI Pipeline",
    icon: <PlayCircle className="w-4 h-4" />,
    description: "GitHub Actions workflows",
  },
  {
    id: "build",
    label: "Container Build",
    icon: <Package className="w-4 h-4" />,
    description: "Roundhouse image build",
  },
  {
    id: "deploy",
    label: "Deployment",
    icon: <Server className="w-4 h-4" />,
    description: "Kubernetes deployment",
  },
];

// ============================================================================
// Utility Functions
// ============================================================================

function formatDuration(startedAt: string, endedAt?: string): string {
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

function getOverallStatusColor(status: OverallPipelineStatus): string {
  switch (status) {
    case "running":
      return "bg-green-500";
    case "ready":
      return "bg-blue-500";
    case "building":
    case "deploying":
      return "bg-yellow-500";
    case "failed":
      return "bg-red-500";
    default:
      return "bg-gray-400";
  }
}

function getOverallStatusLabel(status: OverallPipelineStatus): string {
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

function mapStageStatus(stage: BuildStage): StepStatus {
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

function getStepStatusFromBuildStatus(
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

// ============================================================================
// Sub-components
// ============================================================================

function StepIcon({ status }: { status: StepStatus }) {
  switch (status) {
    case "completed":
      return <CheckCircle2 className="w-6 h-6 text-green-500" />;
    case "active":
      return <Loader2 className="w-6 h-6 text-blue-500 animate-spin" />;
    case "failed":
      return <XCircle className="w-6 h-6 text-red-500" />;
    case "skipped":
      return <Circle className="w-6 h-6 text-gray-400" />;
    default:
      return <Circle className="w-6 h-6 text-muted-foreground" />;
  }
}

function StatusBadge({ status }: { status: OverallPipelineStatus }) {
  const variants: Record<OverallPipelineStatus, string> = {
    pending: "bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-200",
    building: "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200",
    ready: "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200",
    deploying: "bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200",
    running: "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200",
    failed: "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200",
  };

  return (
    <Badge className={cn("font-medium", variants[status])}>
      {getOverallStatusLabel(status)}
    </Badge>
  );
}

interface WorkflowRunItemProps {
  run: CIRun;
}

function WorkflowRunItem({ run }: WorkflowRunItemProps) {
  const statusIcon =
    run.status === "completed" ? (
      run.conclusion === "success" ? (
        <CheckCircle2 className="w-4 h-4 text-green-500" />
      ) : (
        <XCircle className="w-4 h-4 text-red-500" />
      )
    ) : run.status === "in_progress" ? (
      <Loader2 className="w-4 h-4 text-blue-500 animate-spin" />
    ) : (
      <Circle className="w-4 h-4 text-gray-400" />
    );

  return (
    <div className="flex items-center justify-between py-2 px-3 bg-muted/50 rounded-md">
      <div className="flex items-center gap-2">
        {statusIcon}
        <span className="text-sm font-medium">{run.workflow_name}</span>
        {run.run_number && (
          <span className="text-xs text-muted-foreground">#{run.run_number}</span>
        )}
      </div>
      <div className="flex items-center gap-2">
        {run.started_at && (
          <span className="text-xs text-muted-foreground">
            {formatDuration(run.started_at, run.completed_at)}
          </span>
        )}
        {run.html_url && (
          <a
            href={run.html_url}
            target="_blank"
            rel="noopener noreferrer"
            className="text-muted-foreground hover:text-foreground transition-colors"
          >
            <ExternalLink className="w-4 h-4" />
          </a>
        )}
      </div>
    </div>
  );
}

interface StageDetailsProps {
  status: UnifiedBuildStatus;
  stepId: string;
  expanded: boolean;
  onToggle: () => void;
}

function StageDetails({ status, stepId, expanded, onToggle }: StageDetailsProps) {
  const renderCIDetails = () => {
    if (!status.github_actions) return null;
    const { workflows, success_count, failure_count, in_progress, total_runs } =
      status.github_actions;

    return (
      <div className="space-y-2">
        <div className="flex items-center gap-4 text-xs text-muted-foreground">
          <span className="flex items-center gap-1">
            <CheckCircle2 className="w-3 h-3 text-green-500" />
            {success_count} passed
          </span>
          {failure_count > 0 && (
            <span className="flex items-center gap-1">
              <XCircle className="w-3 h-3 text-red-500" />
              {failure_count} failed
            </span>
          )}
          {in_progress > 0 && (
            <span className="flex items-center gap-1">
              <Loader2 className="w-3 h-3 text-blue-500" />
              {in_progress} running
            </span>
          )}
          <span>{total_runs} total</span>
        </div>
        {expanded && workflows.length > 0 && (
          <div className="space-y-1 mt-2">
            {workflows.map((run) => (
              <WorkflowRunItem key={run.id} run={run} />
            ))}
          </div>
        )}
      </div>
    );
  };

  const renderBuildDetails = () => {
    if (!status.roundhouse) return null;
    const { release, has_sbom, has_signature, image_uri, error_message } =
      status.roundhouse;

    return (
      <div className="space-y-2">
        <div className="flex items-center gap-4 text-xs text-muted-foreground">
          {image_uri && (
            <span className="font-mono truncate max-w-[200px]" title={image_uri}>
              {image_uri.split("/").pop()?.substring(0, 20)}...
            </span>
          )}
          {has_sbom && (
            <span className="flex items-center gap-1 text-green-600">
              <FileText className="w-3 h-3" />
              SBOM
            </span>
          )}
          {has_signature && (
            <span className="flex items-center gap-1 text-green-600">
              <Shield className="w-3 h-3" />
              Signed
            </span>
          )}
        </div>
        {expanded && release && (
          <div className="bg-muted/50 rounded-md p-3 text-xs space-y-1">
            <div className="flex justify-between">
              <span className="text-muted-foreground">Version:</span>
              <span className="font-mono">{release.version}</span>
            </div>
            {release.git_sha && (
              <div className="flex justify-between">
                <span className="text-muted-foreground">Commit:</span>
                <span className="font-mono">{release.git_sha.substring(0, 8)}</span>
              </div>
            )}
            {release.created_at && (
              <div className="flex justify-between">
                <span className="text-muted-foreground">Started:</span>
                <span>{new Date(release.created_at).toLocaleTimeString()}</span>
              </div>
            )}
          </div>
        )}
        {error_message && (
          <div className="flex items-start gap-2 p-2 bg-red-500/10 border border-red-500/20 rounded-md">
            <AlertCircle className="w-4 h-4 text-red-500 flex-shrink-0 mt-0.5" />
            <span className="text-sm text-red-600 dark:text-red-400">
              {error_message}
            </span>
          </div>
        )}
      </div>
    );
  };

  const renderDeployDetails = () => {
    if (!status.deployment) return null;
    const { deployment, health, error_message } = status.deployment;

    return (
      <div className="space-y-2">
        <div className="flex items-center gap-4 text-xs text-muted-foreground">
          <span
            className={cn(
              "px-2 py-0.5 rounded-full text-xs font-medium",
              health === "healthy"
                ? "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200"
                : health === "unhealthy"
                  ? "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200"
                  : "bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-200"
            )}
          >
            {health}
          </span>
          {deployment?.replicas !== undefined && (
            <span>
              {deployment.ready_replicas || 0}/{deployment.replicas} replicas
            </span>
          )}
        </div>
        {expanded && deployment && (
          <div className="bg-muted/50 rounded-md p-3 text-xs space-y-1">
            <div className="flex justify-between">
              <span className="text-muted-foreground">ID:</span>
              <span className="font-mono">{deployment.id.substring(0, 8)}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">Status:</span>
              <span className="capitalize">{deployment.status}</span>
            </div>
            {deployment.created_at && (
              <div className="flex justify-between">
                <span className="text-muted-foreground">Started:</span>
                <span>{new Date(deployment.created_at).toLocaleTimeString()}</span>
              </div>
            )}
          </div>
        )}
        {error_message && (
          <div className="flex items-start gap-2 p-2 bg-red-500/10 border border-red-500/20 rounded-md">
            <AlertCircle className="w-4 h-4 text-red-500 flex-shrink-0 mt-0.5" />
            <span className="text-sm text-red-600 dark:text-red-400">
              {error_message}
            </span>
          </div>
        )}
      </div>
    );
  };

  const hasDetails =
    (stepId === "ci" && status.github_actions) ||
    (stepId === "build" && status.roundhouse) ||
    (stepId === "deploy" && status.deployment);

  if (!hasDetails) return null;

  return (
    <div className="mt-2 ml-8">
      <button
        onClick={onToggle}
        className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
      >
        {expanded ? (
          <ChevronDown className="w-3 h-3" />
        ) : (
          <ChevronRight className="w-3 h-3" />
        )}
        {expanded ? "Hide details" : "Show details"}
      </button>
      <div className="mt-2">
        {stepId === "ci" && renderCIDetails()}
        {stepId === "build" && renderBuildDetails()}
        {stepId === "deploy" && renderDeployDetails()}
      </div>
    </div>
  );
}

// ============================================================================
// Main Component
// ============================================================================

export function UnifiedPipelineProgress({
  serviceId,
  commitSha,
  serviceName,
  onComplete,
  onError,
  pollInterval = 5000,
  className,
}: UnifiedPipelineProgressProps) {
  const [status, setStatus] = useState<UnifiedBuildStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [elapsedTime, setElapsedTime] = useState<string>("");
  const [expandedSteps, setExpandedSteps] = useState<Set<string>>(new Set());
  const hasCalledComplete = useRef(false);
  const startTimeRef = useRef<string | null>(null);

  // Fetch status
  const fetchStatus = useCallback(async () => {
    try {
      const data = await apiGet<UnifiedBuildStatus>(
        `/v1/services/${serviceId}/builds/${commitSha}/status`
      );
      setStatus(data);
      setError(null);

      // Track start time from first stage
      if (!startTimeRef.current && data.stages.length > 0) {
        const firstStage = data.stages.find((s) => s.started_at);
        if (firstStage?.started_at) {
          startTimeRef.current = firstStage.started_at;
        }
      }

      // Handle completion
      if (data.overall_status === "running" && !hasCalledComplete.current) {
        hasCalledComplete.current = true;
        onComplete?.();
      }

      // Handle failure
      if (data.overall_status === "failed") {
        const failedStage = data.stages.find((s) => s.status === "failure");
        if (failedStage?.error_message) {
          onError?.(failedStage.error_message);
        }
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to fetch status";
      setError(message);
    } finally {
      setLoading(false);
    }
  }, [serviceId, commitSha, onComplete, onError]);

  // Initial fetch and polling
  useEffect(() => {
    fetchStatus();

    const interval = setInterval(() => {
      if (
        status?.overall_status !== "running" &&
        status?.overall_status !== "failed"
      ) {
        fetchStatus();
      }
    }, pollInterval);

    return () => clearInterval(interval);
  }, [fetchStatus, pollInterval, status?.overall_status]);

  // Update elapsed time
  useEffect(() => {
    if (!startTimeRef.current) return;
    if (status?.overall_status === "running" || status?.overall_status === "failed") {
      const lastStage = [...(status.stages || [])].reverse().find((s) => s.completed_at);
      setElapsedTime(formatDuration(startTimeRef.current, lastStage?.completed_at));
      return;
    }

    const interval = setInterval(() => {
      if (startTimeRef.current) {
        setElapsedTime(formatDuration(startTimeRef.current));
      }
    }, 1000);

    return () => clearInterval(interval);
  }, [status?.overall_status, status?.stages]);

  const toggleStep = (stepId: string) => {
    setExpandedSteps((prev) => {
      const next = new Set(prev);
      if (next.has(stepId)) {
        next.delete(stepId);
      } else {
        next.add(stepId);
      }
      return next;
    });
  };

  const isTerminal =
    status?.overall_status === "running" || status?.overall_status === "failed";

  if (loading && !status) {
    return <UnifiedPipelineProgressSkeleton />;
  }

  if (error && !status) {
    return (
      <Card className={cn("border-red-200 dark:border-red-800", className)}>
        <CardContent className="pt-6">
          <div className="flex items-center gap-2 text-red-600 dark:text-red-400">
            <AlertCircle className="w-5 h-5" />
            <span>{error}</span>
          </div>
          <Button variant="outline" size="sm" className="mt-4" onClick={fetchStatus}>
            Retry
          </Button>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className={className}>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <div className="space-y-1">
            <CardTitle className="text-lg flex items-center gap-2">
              {!isTerminal && <Loader2 className="w-4 h-4 animate-spin text-blue-500" />}
              {isTerminal && status?.overall_status === "running" && (
                <CheckCircle2 className="w-4 h-4 text-green-500" />
              )}
              {isTerminal && status?.overall_status === "failed" && (
                <XCircle className="w-4 h-4 text-red-500" />
              )}
              Pipeline Progress
            </CardTitle>
            <div className="flex items-center gap-3 text-sm text-muted-foreground">
              {serviceName && <span>{serviceName}</span>}
              <span className="flex items-center gap-1 font-mono">
                <GitCommit className="w-3 h-3" />
                {commitSha.substring(0, 8)}
              </span>
            </div>
          </div>
          <div className="flex items-center gap-3">
            {status && <StatusBadge status={status.overall_status} />}
            {elapsedTime && (
              <span className="text-sm text-muted-foreground">{elapsedTime}</span>
            )}
          </div>
        </div>
      </CardHeader>

      <CardContent>
        {/* Pipeline Steps */}
        <div className="space-y-4">
          {PIPELINE_STEPS.map((step, index) => {
            const stepStatus = getStepStatusFromBuildStatus(step.id, status);
            const isLast = index === PIPELINE_STEPS.length - 1;

            return (
              <div key={step.id}>
                <div className="flex items-start gap-3">
                  {/* Icon and connector */}
                  <div className="flex flex-col items-center">
                    <StepIcon status={stepStatus} />
                    {!isLast && (
                      <div
                        className={cn(
                          "w-0.5 h-8 mt-1",
                          stepStatus === "completed"
                            ? "bg-green-500"
                            : stepStatus === "active"
                              ? "bg-blue-500"
                              : "bg-muted"
                        )}
                      />
                    )}
                  </div>

                  {/* Content */}
                  <div className="flex-1 min-w-0 pb-2">
                    <div className="flex items-center gap-2">
                      <span
                        className={cn(
                          "font-medium",
                          stepStatus === "completed"
                            ? "text-green-600 dark:text-green-400"
                            : stepStatus === "active"
                              ? "text-blue-600 dark:text-blue-400"
                              : stepStatus === "failed"
                                ? "text-red-600 dark:text-red-400"
                                : "text-muted-foreground"
                        )}
                      >
                        {step.label}
                      </span>
                      <span className="text-muted-foreground">{step.icon}</span>
                    </div>
                    <p className="text-xs text-muted-foreground mt-0.5">
                      {step.description}
                    </p>

                    {/* Stage details */}
                    {status && (
                      <StageDetails
                        status={status}
                        stepId={step.id}
                        expanded={expandedSteps.has(step.id)}
                        onToggle={() => toggleStep(step.id)}
                      />
                    )}
                  </div>
                </div>
              </div>
            );
          })}
        </div>

        {/* Success Message */}
        {status?.overall_status === "running" && (
          <div className="mt-4 p-3 bg-green-500/10 border border-green-500/20 rounded-md">
            <div className="flex items-center gap-2">
              <CheckCircle2 className="w-4 h-4 text-green-500" />
              <span className="text-sm text-green-600 dark:text-green-400 font-medium">
                Deployment successful! Service is now running.
              </span>
            </div>
          </div>
        )}

        {/* Overall Error Message */}
        {status?.overall_status === "failed" && (
          <div className="mt-4 p-3 bg-red-500/10 border border-red-500/20 rounded-md">
            <div className="flex items-center gap-2">
              <XCircle className="w-4 h-4 text-red-500" />
              <span className="text-sm text-red-600 dark:text-red-400 font-medium">
                Pipeline failed. Check the failed stage for details.
              </span>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

// ============================================================================
// Skeleton
// ============================================================================

export function UnifiedPipelineProgressSkeleton() {
  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <div className="space-y-2">
            <div className="h-5 bg-muted rounded w-40 animate-pulse" />
            <div className="h-4 bg-muted rounded w-32 animate-pulse" />
          </div>
          <div className="flex items-center gap-3">
            <div className="h-6 bg-muted rounded w-20 animate-pulse" />
            <div className="h-4 bg-muted rounded w-12 animate-pulse" />
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div className="space-y-4">
          {[1, 2, 3].map((i) => (
            <div key={i} className="flex items-start gap-3">
              <div className="flex flex-col items-center">
                <div className="w-6 h-6 bg-muted rounded-full animate-pulse" />
                {i < 3 && <div className="w-0.5 h-8 mt-1 bg-muted" />}
              </div>
              <div className="flex-1 pb-2">
                <div className="h-4 bg-muted rounded w-28 animate-pulse" />
                <div className="h-3 bg-muted rounded w-40 mt-1 animate-pulse" />
              </div>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}

export default UnifiedPipelineProgress;
