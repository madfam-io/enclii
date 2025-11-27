"use client";

import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  MoreVertical,
  ExternalLink,
  Settings,
  Box,
  Activity,
  GitBranch
} from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import Link from "next/link";
import { cn } from "@/lib/utils";

interface Service {
  id: string;
  name: string;
  status: "running" | "stopped" | "deploying" | "failed";
  url?: string;
}

interface Project {
  id: string;
  name: string;
  slug: string;
  services: Service[];
  lastDeployment?: {
    timestamp: string;
    status: "success" | "failed" | "pending";
    branch: string;
  };
  usage?: {
    computePercent: number;
    buildPercent: number;
  };
}

interface ProjectCardProps {
  project: Project;
  className?: string;
}

const statusColors = {
  running: "bg-green-500",
  stopped: "bg-gray-400",
  deploying: "bg-blue-500 animate-pulse",
  failed: "bg-red-500",
};

const statusLabels = {
  running: "Running",
  stopped: "Stopped",
  deploying: "Deploying",
  failed: "Failed",
};

export function ProjectCard({ project, className }: ProjectCardProps) {
  const activeServices = project.services.filter(s => s.status === "running").length;

  return (
    <Card className={cn("hover:shadow-md transition-shadow", className)}>
      <CardHeader className="flex flex-row items-start justify-between pb-2">
        <div className="space-y-1">
          <Link
            href={`/projects/${project.slug}`}
            className="font-semibold hover:underline"
          >
            {project.name}
          </Link>
          <p className="text-sm text-muted-foreground">
            {project.slug}
          </p>
        </div>

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="h-8 w-8">
              <MoreVertical className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem asChild>
              <Link href={`/projects/${project.slug}`}>
                <Box className="h-4 w-4 mr-2" />
                View Project
              </Link>
            </DropdownMenuItem>
            <DropdownMenuItem asChild>
              <Link href={`/projects/${project.slug}/settings`}>
                <Settings className="h-4 w-4 mr-2" />
                Settings
              </Link>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </CardHeader>

      <CardContent className="space-y-4">
        {/* Services Summary */}
        <div className="flex items-center gap-2">
          <Activity className="h-4 w-4 text-muted-foreground" />
          <span className="text-sm">
            {activeServices} of {project.services.length} services running
          </span>
        </div>

        {/* Service Status Dots */}
        <div className="flex flex-wrap gap-2">
          {project.services.map((service) => (
            <div
              key={service.id}
              className="flex items-center gap-1.5 bg-muted px-2 py-1 rounded-full"
            >
              <div className={cn(
                "w-2 h-2 rounded-full",
                statusColors[service.status]
              )} />
              <span className="text-xs font-medium">{service.name}</span>
            </div>
          ))}
        </div>

        {/* Last Deployment */}
        {project.lastDeployment && (
          <div className="flex items-center justify-between text-sm border-t pt-3">
            <div className="flex items-center gap-2 text-muted-foreground">
              <GitBranch className="h-4 w-4" />
              <span>{project.lastDeployment.branch}</span>
            </div>
            <Badge
              variant={
                project.lastDeployment.status === "success"
                  ? "default"
                  : project.lastDeployment.status === "failed"
                  ? "destructive"
                  : "secondary"
              }
            >
              {project.lastDeployment.status}
            </Badge>
          </div>
        )}

        {/* Quick Actions */}
        <div className="flex gap-2 pt-2">
          <Button variant="outline" size="sm" className="flex-1" asChild>
            <Link href={`/projects/${project.slug}`}>
              View Details
            </Link>
          </Button>
          {project.services.some(s => s.url) && (
            <Button variant="outline" size="sm" asChild>
              <a
                href={project.services.find(s => s.url)?.url}
                target="_blank"
                rel="noopener noreferrer"
              >
                <ExternalLink className="h-4 w-4" />
              </a>
            </Button>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
