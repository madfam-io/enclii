"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import {
  Cpu,
  HardDrive,
  Gauge,
  Globe,
  Hammer,
  TrendingUp
} from "lucide-react";
import { cn } from "@/lib/utils";

interface UsageMetric {
  type: string;
  label: string;
  used: number;
  included: number;
  unit: string;
  cost: number;
  icon: React.ReactNode;
}

interface UsageMetersProps {
  projectId: string;
  metrics?: UsageMetric[];
  className?: string;
}

const defaultMetrics: UsageMetric[] = [
  {
    type: "compute",
    label: "Compute",
    used: 156.4,
    included: 500,
    unit: "GB-hours",
    cost: 0,
    icon: <Cpu className="h-4 w-4" />,
  },
  {
    type: "build",
    label: "Build Minutes",
    used: 89,
    included: 500,
    unit: "minutes",
    cost: 0,
    icon: <Hammer className="h-4 w-4" />,
  },
  {
    type: "storage",
    label: "Storage",
    used: 2.4,
    included: 10,
    unit: "GB",
    cost: 0,
    icon: <HardDrive className="h-4 w-4" />,
  },
  {
    type: "bandwidth",
    label: "Bandwidth",
    used: 45.2,
    included: 500,
    unit: "GB",
    cost: 0,
    icon: <Gauge className="h-4 w-4" />,
  },
  {
    type: "domains",
    label: "Custom Domains",
    used: 2,
    included: -1, // Unlimited
    unit: "domains",
    cost: 0,
    icon: <Globe className="h-4 w-4" />,
  },
];

function getProgressColor(percentage: number): string {
  if (percentage >= 90) return "bg-red-500";
  if (percentage >= 75) return "bg-yellow-500";
  return "bg-green-500";
}

function formatNumber(num: number): string {
  if (num >= 1000) {
    return (num / 1000).toFixed(1) + "k";
  }
  return num.toFixed(1);
}

export function UsageMeters({
  projectId,
  metrics = defaultMetrics,
  className
}: UsageMetersProps) {
  const totalCost = metrics.reduce((sum, m) => sum + m.cost, 0);

  return (
    <Card className={cn("", className)}>
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <CardTitle className="text-sm font-medium">
          Current Period Usage
        </CardTitle>
        <div className="flex items-center gap-1 text-sm text-muted-foreground">
          <TrendingUp className="h-4 w-4" />
          <span>${totalCost.toFixed(2)} overage</span>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        {metrics.map((metric) => {
          const isUnlimited = metric.included === -1;
          const percentage = isUnlimited
            ? 0
            : Math.min((metric.used / metric.included) * 100, 100);
          const overLimit = !isUnlimited && metric.used > metric.included;

          return (
            <div key={metric.type} className="space-y-2">
              <div className="flex items-center justify-between text-sm">
                <div className="flex items-center gap-2">
                  <span className="text-muted-foreground">{metric.icon}</span>
                  <span className="font-medium">{metric.label}</span>
                </div>
                <div className="flex items-center gap-2">
                  <span className={cn(
                    "font-mono",
                    overLimit && "text-red-500"
                  )}>
                    {formatNumber(metric.used)}
                  </span>
                  <span className="text-muted-foreground">/</span>
                  <span className="text-muted-foreground font-mono">
                    {isUnlimited ? "∞" : formatNumber(metric.included)}
                  </span>
                  <span className="text-muted-foreground text-xs">
                    {metric.unit}
                  </span>
                </div>
              </div>

              {!isUnlimited && (
                <div className="relative">
                  <Progress
                    value={percentage}
                    className="h-2"
                  />
                  <div
                    className={cn(
                      "absolute top-0 left-0 h-2 rounded-full transition-all",
                      getProgressColor(percentage)
                    )}
                    style={{ width: `${Math.min(percentage, 100)}%` }}
                  />
                  {overLimit && (
                    <div
                      className="absolute top-0 h-2 bg-red-500/30 rounded-r-full"
                      style={{
                        left: "100%",
                        width: `${((metric.used - metric.included) / metric.included) * 100}%`,
                        maxWidth: "20%"
                      }}
                    />
                  )}
                </div>
              )}

              {metric.cost > 0 && (
                <p className="text-xs text-muted-foreground">
                  +${metric.cost.toFixed(2)} overage charges
                </p>
              )}
            </div>
          );
        })}
      </CardContent>
    </Card>
  );
}
