import { Metadata } from "next";
import Link from "next/link";
import { ProjectCard } from "@/components/dashboard/project-card";
import { UsageMeters } from "@/components/billing/usage-meters";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Plus,
  Activity,
  Rocket,
  Clock,
  ArrowUpRight,
  CheckCircle2,
  AlertCircle,
  Loader2
} from "lucide-react";

export const metadata: Metadata = {
  title: "Dashboard | Enclii",
  description: "Your Enclii dashboard",
};

// Mock data - would come from API
const projects = [
  {
    id: "1",
    name: "Forgesight",
    slug: "forgesight",
    services: [
      { id: "1", name: "api", status: "running" as const },
      { id: "2", name: "web", status: "running" as const },
      { id: "3", name: "worker", status: "running" as const },
    ],
    lastDeployment: {
      timestamp: "2024-12-01T10:30:00Z",
      status: "success" as const,
      branch: "main",
    },
  },
  {
    id: "2",
    name: "Dhanam",
    slug: "dhanam",
    services: [
      { id: "1", name: "api", status: "running" as const },
      { id: "2", name: "web", status: "deploying" as const },
    ],
    lastDeployment: {
      timestamp: "2024-12-01T09:15:00Z",
      status: "pending" as const,
      branch: "feature/auth",
    },
  },
  {
    id: "3",
    name: "Fortuna",
    slug: "fortuna",
    services: [
      { id: "1", name: "api", status: "stopped" as const },
    ],
    lastDeployment: {
      timestamp: "2024-11-30T15:45:00Z",
      status: "success" as const,
      branch: "main",
    },
  },
];

const recentActivity = [
  {
    id: "1",
    type: "deployment",
    project: "Forgesight",
    message: "Deployed to production",
    timestamp: "5 minutes ago",
    status: "success",
  },
  {
    id: "2",
    type: "build",
    project: "Dhanam",
    message: "Build started for feature/auth",
    timestamp: "10 minutes ago",
    status: "pending",
  },
  {
    id: "3",
    type: "deployment",
    project: "Fortuna",
    message: "Service stopped",
    timestamp: "1 hour ago",
    status: "info",
  },
];

export default function DashboardPage() {
  const totalServices = projects.reduce((sum, p) => sum + p.services.length, 0);
  const runningServices = projects.reduce(
    (sum, p) => sum + p.services.filter(s => s.status === "running").length,
    0
  );

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Dashboard</h1>
          <p className="text-muted-foreground">
            Overview of your projects and services
          </p>
        </div>
        <Button asChild>
          <Link href="/projects/new">
            <Plus className="h-4 w-4 mr-2" />
            New Project
          </Link>
        </Button>
      </div>

      {/* Stats Overview */}
      <div className="grid gap-4 md:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">Projects</CardTitle>
            <Rocket className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{projects.length}</div>
            <p className="text-xs text-muted-foreground">
              Active projects
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">Services</CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {runningServices}/{totalServices}
            </div>
            <p className="text-xs text-muted-foreground">
              Running services
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">Uptime</CardTitle>
            <CheckCircle2 className="h-4 w-4 text-green-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">99.9%</div>
            <p className="text-xs text-muted-foreground">
              Last 30 days
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">This Month</CardTitle>
            <Clock className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">$24.50</div>
            <p className="text-xs text-muted-foreground">
              Estimated cost
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Main Content Grid */}
      <div className="grid gap-6 lg:grid-cols-3">
        {/* Projects */}
        <div className="lg:col-span-2 space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-semibold">Projects</h2>
            <Button variant="ghost" size="sm" asChild>
              <Link href="/projects">
                View All
                <ArrowUpRight className="h-4 w-4 ml-1" />
              </Link>
            </Button>
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            {projects.map((project) => (
              <ProjectCard key={project.id} project={project} />
            ))}
          </div>
        </div>

        {/* Sidebar */}
        <div className="space-y-6">
          {/* Usage Summary */}
          <UsageMeters projectId="all" />

          {/* Recent Activity */}
          <Card>
            <CardHeader>
              <CardTitle className="text-sm font-medium">
                Recent Activity
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              {recentActivity.map((activity) => (
                <div
                  key={activity.id}
                  className="flex items-start gap-3"
                >
                  <div className="mt-0.5">
                    {activity.status === "success" && (
                      <CheckCircle2 className="h-4 w-4 text-green-500" />
                    )}
                    {activity.status === "pending" && (
                      <Loader2 className="h-4 w-4 text-blue-500 animate-spin" />
                    )}
                    {activity.status === "info" && (
                      <AlertCircle className="h-4 w-4 text-muted-foreground" />
                    )}
                  </div>
                  <div className="flex-1 space-y-1">
                    <p className="text-sm">
                      <span className="font-medium">{activity.project}</span>
                      {" - "}
                      {activity.message}
                    </p>
                    <p className="text-xs text-muted-foreground">
                      {activity.timestamp}
                    </p>
                  </div>
                </div>
              ))}
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
