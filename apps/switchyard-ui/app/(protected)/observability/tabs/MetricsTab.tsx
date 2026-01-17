"use client";

/**
 * MetricsTab
 * Displays metrics charts and system statistics
 */

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  AreaChart,
  Area,
} from "recharts";
import type { MetricsSnapshot, MetricsHistory } from "../observability-types";

interface MetricsTabProps {
  snapshot: MetricsSnapshot | null;
  history: MetricsHistory | null;
  timeRange: string;
}

export function MetricsTab({ snapshot, history, timeRange }: MetricsTabProps) {
  const formatTime = (timestamp: string) => {
    const date = new Date(timestamp);
    return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  };

  return (
    <div className="space-y-6">
      {/* Latency Chart */}
      <Card>
        <CardHeader>
          <CardTitle>Response Latency</CardTitle>
          <CardDescription>Average response time over {timeRange}</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="h-80">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={history?.data_points || []}>
                <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                <XAxis dataKey="timestamp" tickFormatter={formatTime} />
                <YAxis unit="ms" />
                <Tooltip
                  labelFormatter={(label) => new Date(label).toLocaleString()}
                  formatter={(value: number) => [`${value.toFixed(2)}ms`, "Latency"]}
                />
                <Area
                  type="monotone"
                  dataKey="average_latency_ms"
                  className="fill-status-info/30 stroke-status-info"
                  strokeWidth={2}
                  name="Latency"
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </CardContent>
      </Card>

      {/* Error Rate Chart */}
      <Card>
        <CardHeader>
          <CardTitle>Error Rate</CardTitle>
          <CardDescription>
            Percentage of failed requests over {timeRange}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="h-80">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={history?.data_points || []}>
                <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                <XAxis dataKey="timestamp" tickFormatter={formatTime} />
                <YAxis unit="%" domain={[0, "auto"]} />
                <Tooltip
                  labelFormatter={(label) => new Date(label).toLocaleString()}
                  formatter={(value: number) => [
                    `${(value * 100).toFixed(2)}%`,
                    "Error Rate",
                  ]}
                />
                <Line
                  type="monotone"
                  dataKey="error_rate"
                  className="stroke-status-error"
                  strokeWidth={2}
                  dot={false}
                  name="Error Rate"
                />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </CardContent>
      </Card>

      {/* System Metrics */}
      <div className="grid gap-6 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Database</CardTitle>
            <CardDescription>Connection pool and query metrics</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">
                  Connections Open
                </span>
                <span className="font-mono font-medium">
                  {snapshot?.database.connections_open || 0}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">
                  Connections In Use
                </span>
                <span className="font-mono font-medium">
                  {snapshot?.database.connections_in_use || 0}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Avg Query Time</span>
                <span className="font-mono font-medium">
                  {((snapshot?.database.average_query_time || 0) * 1000).toFixed(2)}ms
                </span>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Cache</CardTitle>
            <CardDescription>Cache hit rate and performance</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Hit Rate</span>
                <span
                  className={`font-mono font-medium ${
                    (snapshot?.cache.hit_rate || 0) < 0.8
                      ? "text-status-warning"
                      : "text-status-success"
                  }`}
                >
                  {((snapshot?.cache.hit_rate || 0) * 100).toFixed(1)}%
                </span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Avg Latency</span>
                <span className="font-mono font-medium">
                  {((snapshot?.cache.average_latency || 0) * 1000).toFixed(2)}ms
                </span>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
