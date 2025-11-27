"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  PlayCircle,
  StopCircle,
  RefreshCw,
  ExternalLink,
  Terminal,
  Settings,
  Clock,
  Cpu,
  HardDrive
} from "lucide-react";
import { cn } from "@/lib/utils";

interface Deployment {
  id: string;
  version: string;
  status: "running" | "stopped" | "deploying" | "failed";
  health: "healthy" | "unhealthy" | "unknown";
  replicas: number;
  cpu: string;
  memory: string;
  uptime: string;
  url?: string;
}

interface Service {
  id: string;
  name: string;
  type: "web" | "worker" | "cron";
  environment: string;
  deployment?: Deployment;
  lastBuild?: {
    timestamp: string;
    duration: string;
    status: "success" | "failed";
  };
}

interface ServiceStatusProps {
  service: Service;
  onRestart?: () => void;
  onStop?: () => void;
  onDeploy?: () => void;
  className?: string;
}

const statusConfig = {
  running: { color: "bg-green-500", label: "Running", icon: PlayCircle },
  stopped: { color: "bg-gray-400", label: "Stopped", icon: StopCircle },
  deploying: { color: "bg-blue-500 animate-pulse", label: "Deploying", icon: RefreshCw },
  failed: { color: "bg-red-500", label: "Failed", icon: StopCircle },
};

const healthConfig = {
  healthy: { color: "text-green-500", label: "Healthy" },
  unhealthy: { color: "text-red-500", label: "Unhealthy" },
  unknown: { color: "text-gray-400", label: "Unknown" },
};

export function ServiceStatus({
  service,
  onRestart,
  onStop,
  onDeploy,
  className
}: ServiceStatusProps) {
  const deployment = service.deployment;
  const status = deployment?.status || "stopped";
  const health = deployment?.health || "unknown";

  const StatusIcon = statusConfig[status].icon;

  return (
    <Card className={cn("", className)}>
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <div className="space-y-1">
          <CardTitle className="text-base font-medium flex items-center gap-2">
            {service.name}
            <Badge variant="outline" className="font-normal">
              {service.type}
            </Badge>
          </CardTitle>
          <p className="text-sm text-muted-foreground">
            {service.environment}
          </p>
        </div>

        <div className="flex items-center gap-2">
          <div className={cn(
            "w-3 h-3 rounded-full",
            statusConfig[status].color
          )} />
          <span className="text-sm font-medium">
            {statusConfig[status].label}
          </span>
        </div>
      </CardHeader>

      <CardContent className="space-y-4">
        {deployment && (
          <>
            {/* Deployment Info */}
            <div className="grid grid-cols-2 gap-4 text-sm">
              <div className="space-y-1">
                <p className="text-muted-foreground">Version</p>
                <p className="font-mono">{deployment.version}</p>
              </div>
              <div className="space-y-1">
                <p className="text-muted-foreground">Health</p>
                <p className={cn("font-medium", healthConfig[health].color)}>
                  {healthConfig[health].label}
                </p>
              </div>
              <div className="space-y-1">
                <p className="text-muted-foreground">Replicas</p>
                <p>{deployment.replicas}</p>
              </div>
              <div className="space-y-1">
                <p className="text-muted-foreground">Uptime</p>
                <p className="flex items-center gap-1">
                  <Clock className="h-3 w-3" />
                  {deployment.uptime}
                </p>
              </div>
            </div>

            {/* Resources */}
            <div className="flex items-center gap-4 text-sm border-t pt-3">
              <div className="flex items-center gap-1 text-muted-foreground">
                <Cpu className="h-4 w-4" />
                <span>{deployment.cpu}</span>
              </div>
              <div className="flex items-center gap-1 text-muted-foreground">
                <HardDrive className="h-4 w-4" />
                <span>{deployment.memory}</span>
              </div>
            </div>
          </>
        )}

        {/* Last Build */}
        {service.lastBuild && (
          <div className="flex items-center justify-between text-sm border-t pt-3">
            <span className="text-muted-foreground">Last build</span>
            <div className="flex items-center gap-2">
              <span>{service.lastBuild.duration}</span>
              <Badge
                variant={service.lastBuild.status === "success" ? "default" : "destructive"}
                className="text-xs"
              >
                {service.lastBuild.status}
              </Badge>
            </div>
          </div>
        )}

        {/* Actions */}
        <div className="flex items-center gap-2 pt-2 border-t">
          {status === "running" ? (
            <>
              <Button
                variant="outline"
                size="sm"
                onClick={onRestart}
              >
                <RefreshCw className="h-4 w-4 mr-1" />
                Restart
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={onStop}
              >
                <StopCircle className="h-4 w-4 mr-1" />
                Stop
              </Button>
            </>
          ) : (
            <Button
              variant="outline"
              size="sm"
              onClick={onDeploy}
            >
              <PlayCircle className="h-4 w-4 mr-1" />
              Deploy
            </Button>
          )}

          <div className="flex-1" />

          <Button variant="ghost" size="icon" className="h-8 w-8">
            <Terminal className="h-4 w-4" />
          </Button>

          {deployment?.url && (
            <Button variant="ghost" size="icon" className="h-8 w-8" asChild>
              <a href={deployment.url} target="_blank" rel="noopener noreferrer">
                <ExternalLink className="h-4 w-4" />
              </a>
            </Button>
          )}

          <Button variant="ghost" size="icon" className="h-8 w-8">
            <Settings className="h-4 w-4" />
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
