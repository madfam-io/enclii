"use client";

import { useState, useEffect, useRef } from "react";
import { CheckCircle2, Circle, Loader2, XCircle } from "lucide-react";
import { cn } from "@/lib/utils";

export type DeploymentStage =
  | "queued"
  | "building"
  | "pushing"
  | "deploying"
  | "verifying"
  | "completed"
  | "failed";

interface DeploymentStep {
  id: string;
  label: string;
  stage: DeploymentStage;
}

const DEPLOYMENT_STEPS: DeploymentStep[] = [
  { id: "build", label: "Building", stage: "building" },
  { id: "push", label: "Pushing Image", stage: "pushing" },
  { id: "deploy", label: "Deploying", stage: "deploying" },
  { id: "verify", label: "Verifying", stage: "verifying" },
];

interface DeploymentProgressProps {
  releaseId: string;
  serviceName: string;
  currentStage: DeploymentStage;
  startedAt?: string;
  completedAt?: string;
  error?: string;
  onComplete?: () => void;
}

function getStepStatus(
  stepStage: DeploymentStage,
  currentStage: DeploymentStage
): "pending" | "active" | "completed" | "failed" {
  const stageOrder: DeploymentStage[] = [
    "queued",
    "building",
    "pushing",
    "deploying",
    "verifying",
    "completed",
  ];

  if (currentStage === "failed") {
    const currentIndex = stageOrder.indexOf(stepStage);
    const failedIndex = stageOrder.indexOf("verifying"); // Assume failed at latest active step
    if (currentIndex < failedIndex) return "completed";
    if (currentIndex === failedIndex) return "failed";
    return "pending";
  }

  const stepIndex = stageOrder.indexOf(stepStage);
  const currentIndex = stageOrder.indexOf(currentStage);

  if (stepIndex < currentIndex) return "completed";
  if (stepIndex === currentIndex) return "active";
  return "pending";
}

function StepIcon({
  status,
}: {
  status: "pending" | "active" | "completed" | "failed";
}) {
  switch (status) {
    case "completed":
      return <CheckCircle2 className="w-5 h-5 text-green-500" />;
    case "active":
      return <Loader2 className="w-5 h-5 text-blue-500 animate-spin" />;
    case "failed":
      return <XCircle className="w-5 h-5 text-red-500" />;
    default:
      return <Circle className="w-5 h-5 text-muted-foreground" />;
  }
}

function formatDuration(startedAt: string, endedAt?: string): string {
  const start = new Date(startedAt);
  const end = endedAt ? new Date(endedAt) : new Date();
  const diffMs = end.getTime() - start.getTime();
  const diffSec = Math.floor(diffMs / 1000);

  if (diffSec < 60) return `${diffSec}s`;
  const minutes = Math.floor(diffSec / 60);
  const seconds = diffSec % 60;
  return `${minutes}m ${seconds}s`;
}

/**
 * DeploymentProgress - Visual progress indicator for deployments
 *
 * Shows build → push → deploy → verify stages with real-time updates.
 */
export function DeploymentProgress({
  releaseId,
  serviceName,
  currentStage,
  startedAt,
  completedAt,
  error,
  onComplete,
}: DeploymentProgressProps) {
  const [elapsedTime, setElapsedTime] = useState<string>("");
  const hasCalledComplete = useRef(false);

  // Update elapsed time every second during active deployment
  useEffect(() => {
    if (!startedAt) return;
    if (currentStage === "completed" || currentStage === "failed") {
      setElapsedTime(formatDuration(startedAt, completedAt));
      return;
    }

    const interval = setInterval(() => {
      setElapsedTime(formatDuration(startedAt));
    }, 1000);

    return () => clearInterval(interval);
  }, [startedAt, completedAt, currentStage]);

  // Call onComplete when deployment finishes
  useEffect(() => {
    if (
      currentStage === "completed" &&
      onComplete &&
      !hasCalledComplete.current
    ) {
      hasCalledComplete.current = true;
      onComplete();
    }
  }, [currentStage, onComplete]);

  const isActive =
    currentStage !== "queued" &&
    currentStage !== "completed" &&
    currentStage !== "failed";

  return (
    <div
      className="bg-card border border-border rounded-lg p-4"
      data-testid="deployment-progress"
    >
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <div>
          <h4 className="text-sm font-medium text-foreground">
            Deploying {serviceName}
          </h4>
          <p className="text-xs text-muted-foreground font-mono">
            Release: {releaseId.substring(0, 8)}
          </p>
        </div>
        {startedAt && (
          <div className="text-right">
            <p className="text-sm font-medium text-foreground">{elapsedTime}</p>
            <p className="text-xs text-muted-foreground">
              {currentStage === "completed"
                ? "Completed"
                : currentStage === "failed"
                  ? "Failed"
                  : "In Progress"}
            </p>
          </div>
        )}
      </div>

      {/* Progress Steps */}
      <div className="flex items-center justify-between">
        {DEPLOYMENT_STEPS.map((step, index) => {
          const status = getStepStatus(step.stage, currentStage);
          const isLast = index === DEPLOYMENT_STEPS.length - 1;

          return (
            <div key={step.id} className="flex items-center flex-1">
              <div className="flex flex-col items-center">
                <StepIcon status={status} />
                <span
                  className={cn(
                    "text-xs mt-1 whitespace-nowrap",
                    status === "active"
                      ? "text-blue-500 font-medium"
                      : status === "completed"
                        ? "text-green-500"
                        : status === "failed"
                          ? "text-red-500"
                          : "text-muted-foreground"
                  )}
                >
                  {step.label}
                </span>
              </div>
              {!isLast && (
                <div
                  className={cn(
                    "flex-1 h-0.5 mx-2 mt-[-1rem]",
                    status === "completed"
                      ? "bg-green-500"
                      : status === "active"
                        ? "bg-blue-500"
                        : "bg-muted"
                  )}
                />
              )}
            </div>
          );
        })}
      </div>

      {/* Error Message */}
      {error && currentStage === "failed" && (
        <div className="mt-4 p-3 bg-red-500/10 border border-red-500/20 rounded-md">
          <p className="text-sm text-red-500">{error}</p>
        </div>
      )}

      {/* Success Message */}
      {currentStage === "completed" && (
        <div className="mt-4 p-3 bg-green-500/10 border border-green-500/20 rounded-md">
          <p className="text-sm text-green-500">
            Deployment completed successfully!
          </p>
        </div>
      )}
    </div>
  );
}

/**
 * DeploymentProgressSkeleton - Loading state for DeploymentProgress
 */
export function DeploymentProgressSkeleton() {
  return (
    <div
      className="bg-card border border-border rounded-lg p-4"
      data-testid="deployment-progress-skeleton"
    >
      <div className="flex items-center justify-between mb-4">
        <div>
          <div className="h-4 bg-muted rounded w-32 animate-pulse" />
          <div className="h-3 bg-muted rounded w-24 mt-1 animate-pulse" />
        </div>
        <div className="text-right">
          <div className="h-4 bg-muted rounded w-16 animate-pulse" />
          <div className="h-3 bg-muted rounded w-20 mt-1 animate-pulse" />
        </div>
      </div>
      <div className="flex items-center justify-between">
        {[1, 2, 3, 4].map((i) => (
          <div key={i} className="flex items-center flex-1">
            <div className="flex flex-col items-center">
              <div className="w-5 h-5 bg-muted rounded-full animate-pulse" />
              <div className="h-3 bg-muted rounded w-12 mt-1 animate-pulse" />
            </div>
            {i < 4 && <div className="flex-1 h-0.5 mx-2 mt-[-1rem] bg-muted" />}
          </div>
        ))}
      </div>
    </div>
  );
}
