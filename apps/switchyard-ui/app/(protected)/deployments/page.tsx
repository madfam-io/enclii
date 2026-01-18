'use client';

import { useState, useEffect } from "react";
import Link from "next/link";
import { RefreshCw } from "lucide-react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { DeploymentProgress, DeploymentProgressSkeleton, type DeploymentStage } from "@/components/dashboard/deployment-progress";
import { apiGet } from "@/lib/api";

interface RecentActivity {
  id: string;
  type: string;
  message: string;
  timestamp: string;
  status: "success" | "running" | "failed" | "pending";
  metadata?: {
    version?: string;
    environment?: string;
    service_name?: string;
    project_name?: string;
  };
}

interface DashboardResponse {
  stats: any;
  activities: RecentActivity[];
  services: any[];
}

export default function DeploymentsPage() {
  const [deployments, setDeployments] = useState<RecentActivity[]>([]);
  const [activeDeployments, setActiveDeployments] = useState<RecentActivity[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchDeployments = async (isManualRefresh = false) => {
    try {
      setError(null);
      if (isManualRefresh) {
        setRefreshing(true);
      }
      const data = await apiGet<DashboardResponse>(`/v1/dashboard/stats`);
      // Filter for deployment-related activities
      const deploymentActivities = (data.activities || []).filter(
        (a) => a.type === "deployment" || a.type === "deploy" || a.message.toLowerCase().includes("deploy")
      );
      const allDeployments = deploymentActivities.length > 0 ? deploymentActivities : data.activities || [];

      // Separate active (running) deployments from history
      const active = allDeployments.filter((d) => d.status === "running");
      const history = allDeployments.filter((d) => d.status !== "running");

      setActiveDeployments(active);
      setDeployments(history);
      setLoading(false);
      setRefreshing(false);
    } catch (err) {
      console.error("Failed to fetch deployments:", err);
      setError(err instanceof Error ? err.message : "Failed to fetch deployments");
      setLoading(false);
      setRefreshing(false);
    }
  };

  // Map activity status to DeploymentStage
  const getDeploymentStage = (activity: RecentActivity): DeploymentStage => {
    const message = activity.message.toLowerCase();
    if (message.includes("building") || message.includes("build")) return "building";
    if (message.includes("pushing") || message.includes("push")) return "pushing";
    if (message.includes("deploying") || message.includes("deploy")) return "deploying";
    if (message.includes("verifying") || message.includes("verify")) return "verifying";
    if (activity.status === "success") return "completed";
    if (activity.status === "failed") return "failed";
    return "deploying"; // default for running
  };

  useEffect(() => {
    fetchDeployments();

    // Refresh every 30 seconds
    const interval = setInterval(fetchDeployments, 30000);
    return () => clearInterval(interval);
  }, []);

  const formatTimeAgo = (timestamp: string) => {
    const now = new Date();
    const time = new Date(timestamp);
    const diffInSeconds = Math.floor((now.getTime() - time.getTime()) / 1000);

    if (diffInSeconds < 60) {
      return `${diffInSeconds} seconds ago`;
    } else if (diffInSeconds < 3600) {
      const minutes = Math.floor(diffInSeconds / 60);
      return `${minutes} minute${minutes > 1 ? "s" : ""} ago`;
    } else if (diffInSeconds < 86400) {
      const hours = Math.floor(diffInSeconds / 3600);
      return `${hours} hour${hours > 1 ? "s" : ""} ago`;
    } else {
      const days = Math.floor(diffInSeconds / 86400);
      return `${days} day${days > 1 ? "s" : ""} ago`;
    }
  };

  const getStatusBadgeClass = (status: string) => {
    switch (status) {
      case "success":
        return "bg-status-success-muted text-status-success-foreground";
      case "running":
        return "bg-status-info-muted text-status-info-foreground";
      case "failed":
        return "bg-status-error-muted text-status-error-foreground";
      default:
        return "bg-status-warning-muted text-status-warning-foreground";
    }
  };

  const getStatusDotClass = (status: string) => {
    switch (status) {
      case "success":
        return "bg-status-success";
      case "running":
        return "bg-status-info animate-pulse";
      case "failed":
        return "bg-status-error";
      default:
        return "bg-status-warning";
    }
  };

  if (loading) {
    return (
      <div className="container mx-auto py-8">
        <div className="mb-8">
          <h1 className="text-3xl font-bold">Deployments</h1>
          <p className="text-muted-foreground mt-2">
            Track and manage your deployment history
          </p>
        </div>
        <Card>
          <CardContent className="py-12">
            <div className="flex items-center justify-center">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
              <span className="ml-3 text-muted-foreground">Loading deployments...</span>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  if (error) {
    return (
      <div className="container mx-auto py-8">
        <div className="mb-8">
          <h1 className="text-3xl font-bold">Deployments</h1>
          <p className="text-muted-foreground mt-2">
            Track and manage your deployment history
          </p>
        </div>
        <Card className="border-status-error/30 bg-status-error-muted">
          <CardContent className="py-8">
            <div className="text-center">
              <p className="text-status-error font-medium mb-4">{error}</p>
              <button
                onClick={fetchDeployments}
                className="inline-flex items-center px-4 py-2 border border-status-error/30 rounded-md shadow-sm text-sm font-medium text-status-error-foreground bg-white hover:bg-status-error-muted"
              >
                Try Again
              </button>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="container mx-auto py-8">
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-3xl font-bold">Deployments</h1>
          <p className="text-muted-foreground mt-2">
            Track and manage your deployment history
          </p>
        </div>
        <button
          onClick={() => fetchDeployments(true)}
          disabled={refreshing}
          className="inline-flex items-center px-4 py-2 border border-input rounded-md shadow-sm text-sm font-medium text-foreground bg-background hover:bg-accent disabled:opacity-50 disabled:cursor-not-allowed"
        >
          <RefreshCw className={`w-4 h-4 mr-2 ${refreshing ? 'animate-spin' : ''}`} />
          {refreshing ? 'Refreshing...' : 'Refresh'}
        </button>
      </div>

      {/* Active Deployments */}
      {activeDeployments.length > 0 && (
        <div className="mb-8 space-y-4">
          <h2 className="text-lg font-semibold text-foreground">Active Deployments</h2>
          {activeDeployments.map((deployment) => (
            <DeploymentProgress
              key={deployment.id}
              releaseId={deployment.id}
              serviceName={deployment.metadata?.service_name || "Unknown Service"}
              currentStage={getDeploymentStage(deployment)}
              startedAt={deployment.timestamp}
              onComplete={fetchDeployments}
            />
          ))}
        </div>
      )}

      <Card>
        <CardHeader>
          <CardTitle>Deployment History</CardTitle>
          <CardDescription>
            View all deployments across your services ({deployments.length} total)
          </CardDescription>
        </CardHeader>
        <CardContent>
          {deployments.length === 0 ? (
            <div className="text-center py-12">
              <div className="mx-auto w-12 h-12 rounded-full bg-gray-100 flex items-center justify-center mb-4">
                <svg className="w-6 h-6 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 8h14M5 8a2 2 0 110-4h14a2 2 0 110 4M5 8v10a2 2 0 002 2h10a2 2 0 002-2V8m-9 4h4" />
                </svg>
              </div>
              <p className="text-lg font-medium text-gray-900">No deployments found</p>
              <p className="text-sm text-muted-foreground mt-2 max-w-md mx-auto">
                Once you deploy a service, your deployment history will appear here.
              </p>
              <div className="mt-6 space-y-2 text-left max-w-md mx-auto bg-gray-50 rounded-lg p-4">
                <p className="text-sm font-medium text-gray-700">Possible reasons:</p>
                <ul className="text-sm text-muted-foreground space-y-1 list-disc list-inside">
                  <li>No services have been registered yet</li>
                  <li>Services exist but have no deployments</li>
                  <li>Webhook hasn&apos;t triggered a build yet</li>
                </ul>
                <p className="text-sm text-muted-foreground mt-3">
                  Check the{" "}
                  <Link href="/services" className="text-blue-600 hover:underline">
                    Services page
                  </Link>{" "}
                  to verify your services are registered.
                </p>
              </div>
            </div>
          ) : (
            <div className="space-y-4">
              {deployments.map((deployment) => (
                <div
                  key={deployment.id}
                  className="flex items-center justify-between p-4 bg-gray-50 rounded-lg hover:bg-gray-100 transition-colors"
                >
                  <div className="flex items-center space-x-4">
                    <div className={`w-3 h-3 rounded-full ${getStatusDotClass(deployment.status)}`}></div>
                    <div>
                      <p className="font-medium text-gray-900">{deployment.message}</p>
                      <div className="flex items-center space-x-2 text-sm text-gray-500 mt-1">
                        {deployment.metadata?.service_name && (
                          <span className="font-medium">{deployment.metadata.service_name}</span>
                        )}
                        {deployment.metadata?.version && (
                          <>
                            <span>•</span>
                            <span>{deployment.metadata.version}</span>
                          </>
                        )}
                        {deployment.metadata?.environment && (
                          <>
                            <span>•</span>
                            <span className="capitalize">{deployment.metadata.environment}</span>
                          </>
                        )}
                        <span>•</span>
                        <span>{formatTimeAgo(deployment.timestamp)}</span>
                      </div>
                    </div>
                  </div>
                  <span
                    className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusBadgeClass(deployment.status)}`}
                  >
                    {deployment.status.charAt(0).toUpperCase() + deployment.status.slice(1)}
                  </span>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
