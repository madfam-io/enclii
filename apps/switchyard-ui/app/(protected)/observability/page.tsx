'use client';

import { useState, useEffect, useCallback } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
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
} from 'recharts';
import { apiGet } from '@/lib/api';

// Types
interface MetricsSnapshot {
  timestamp: string;
  http: {
    requests_per_second: number;
    average_latency: number;
    error_rate: number;
  };
  database: {
    connections_open: number;
    connections_in_use: number;
    average_query_time: number;
    error_rate: number;
  };
  cache: {
    hit_rate: number;
    average_latency: number;
    operations_per_second: number;
  };
  builds: {
    success_rate: number;
    average_duration: number;
    queue_length: number;
  };
  kubernetes: {
    operation_latency: number;
    error_rate: number;
    active_pods: number;
  };
}

interface MetricsDataPoint {
  timestamp: string;
  requests_per_sec: number;
  average_latency_ms: number;
  error_rate: number;
  cpu_usage: number;
  memory_usage: number;
}

interface MetricsHistory {
  time_range: string;
  resolution: string;
  data_points: MetricsDataPoint[];
}

interface ServiceHealth {
  service_id: string;
  service_name: string;
  project_slug: string;
  status: string;
  uptime: number;
  response_time_ms: number;
  error_rate: number;
  last_checked: string;
  pod_count: number;
  ready_pods: number;
}

interface ServiceHealthResponse {
  services: ServiceHealth[];
  healthy_count: number;
  degraded_count: number;
  unhealthy_count: number;
  timestamp: string;
}

interface ErrorEntry {
  id: string;
  timestamp: string;
  service_id: string;
  service_name: string;
  level: string;
  message: string;
  stack_trace?: string;
  count: number;
  last_seen: string;
  first_seen: string;
  resolved: boolean;
}

interface RecentErrorsResponse {
  errors: ErrorEntry[];
  total_count: number;
  time_range: string;
}

interface Alert {
  id: string;
  name: string;
  severity: string;
  status: string;
  message: string;
  service_id?: string;
  service_name?: string;
  value?: number;
  threshold?: number;
  fired_at: string;
  resolved_at?: string;
  labels?: Record<string, string>;
}

interface AlertsResponse {
  alerts: Alert[];
  critical_count: number;
  warning_count: number;
  info_count: number;
  timestamp: string;
}

type Tab = 'metrics' | 'health' | 'errors' | 'alerts';

export default function ObservabilityPage() {
  const [activeTab, setActiveTab] = useState<Tab>('metrics');
  const [timeRange, setTimeRange] = useState('1h');
  const [snapshot, setSnapshot] = useState<MetricsSnapshot | null>(null);
  const [history, setHistory] = useState<MetricsHistory | null>(null);
  const [serviceHealth, setServiceHealth] = useState<ServiceHealthResponse | null>(null);
  const [errors, setErrors] = useState<RecentErrorsResponse | null>(null);
  const [alerts, setAlerts] = useState<AlertsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    try {
      setError(null);
      setLoading(true);

      const [snapshotData, historyData, healthData, errorsData, alertsData] = await Promise.all([
        apiGet<MetricsSnapshot>('/v1/observability/metrics'),
        apiGet<MetricsHistory>(`/v1/observability/metrics/history?range=${timeRange}`),
        apiGet<ServiceHealthResponse>('/v1/observability/health'),
        apiGet<RecentErrorsResponse>('/v1/observability/errors?limit=50'),
        apiGet<AlertsResponse>('/v1/observability/alerts'),
      ]);

      setSnapshot(snapshotData);
      setHistory(historyData);
      setServiceHealth(healthData);
      setErrors(errorsData);
      setAlerts(alertsData);
    } catch (err) {
      console.error('Failed to fetch observability data:', err);
      setError(err instanceof Error ? err.message : 'Failed to fetch observability data');
    } finally {
      setLoading(false);
    }
  }, [timeRange]);

  useEffect(() => {
    fetchData();
    // Auto-refresh every 30 seconds
    const interval = setInterval(fetchData, 30000);
    return () => clearInterval(interval);
  }, [fetchData]);

  const formatTime = (timestamp: string) => {
    const date = new Date(timestamp);
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'healthy': return 'bg-green-100 text-green-800';
      case 'degraded': return 'bg-yellow-100 text-yellow-800';
      case 'unhealthy': return 'bg-red-100 text-red-800';
      default: return 'bg-gray-100 text-gray-800';
    }
  };

  const getSeverityColor = (severity: string) => {
    switch (severity) {
      case 'critical': return 'bg-red-100 text-red-800 border-red-200';
      case 'warning': return 'bg-yellow-100 text-yellow-800 border-yellow-200';
      case 'info': return 'bg-blue-100 text-blue-800 border-blue-200';
      default: return 'bg-gray-100 text-gray-800 border-gray-200';
    }
  };

  const tabs = [
    { id: 'metrics' as Tab, label: 'Metrics', icon: (
      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
      </svg>
    )},
    { id: 'health' as Tab, label: 'Service Health', icon: (
      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z" />
      </svg>
    )},
    { id: 'errors' as Tab, label: 'Errors', icon: (
      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
    )},
    { id: 'alerts' as Tab, label: 'Alerts', icon: (
      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
      </svg>
    )},
  ];

  if (loading && !snapshot) {
    return (
      <div className="flex items-center justify-center py-24">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
        <span className="ml-3 text-muted-foreground">Loading observability data...</span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="text-center py-24">
        <p className="text-red-600 mb-4">{error}</p>
        <Button variant="outline" onClick={fetchData}>
          Try Again
        </Button>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Observability</h1>
          <p className="text-muted-foreground">
            Monitor metrics, health, errors, and alerts across your services
          </p>
        </div>
        <div className="flex items-center gap-4">
          <select
            value={timeRange}
            onChange={(e) => setTimeRange(e.target.value)}
            className="px-3 py-2 border rounded-md bg-white text-sm"
          >
            <option value="1h">Last 1 hour</option>
            <option value="6h">Last 6 hours</option>
            <option value="24h">Last 24 hours</option>
            <option value="7d">Last 7 days</option>
          </select>
          <Button variant="outline" onClick={fetchData}>
            <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
            Refresh
          </Button>
        </div>
      </div>

      {/* Summary Cards */}
      <div className="grid gap-4 md:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Error Rate</CardTitle>
            <svg className="w-4 h-4 text-muted-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {((snapshot?.http.error_rate || 0) * 100).toFixed(2)}%
            </div>
            <p className="text-xs text-muted-foreground">HTTP request error rate</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Avg Latency</CardTitle>
            <svg className="w-4 h-4 text-muted-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {((snapshot?.http.average_latency || 0) * 1000).toFixed(0)}ms
            </div>
            <p className="text-xs text-muted-foreground">Average response time</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Healthy Services</CardTitle>
            <svg className="w-4 h-4 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-600">
              {serviceHealth?.healthy_count || 0}/{(serviceHealth?.services.length || 0)}
            </div>
            <p className="text-xs text-muted-foreground">Services running healthy</p>
          </CardContent>
        </Card>
        <Card className={alerts && alerts.critical_count > 0 ? 'border-red-200 bg-red-50' : ''}>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Active Alerts</CardTitle>
            <svg className={`w-4 h-4 ${alerts && alerts.critical_count > 0 ? 'text-red-500' : 'text-muted-foreground'}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
            </svg>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {alerts?.alerts.length || 0}
            </div>
            <p className="text-xs text-muted-foreground">
              {alerts?.critical_count || 0} critical, {alerts?.warning_count || 0} warning
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Tab Navigation */}
      <div className="border-b">
        <nav className="flex space-x-8" aria-label="Tabs">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`flex items-center gap-2 py-4 px-1 border-b-2 font-medium text-sm ${
                activeTab === tab.id
                  ? 'border-blue-500 text-blue-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
              }`}
            >
              {tab.icon}
              {tab.label}
              {tab.id === 'alerts' && alerts && alerts.alerts.length > 0 && (
                <Badge variant={alerts.critical_count > 0 ? 'destructive' : 'secondary'} className="ml-1">
                  {alerts.alerts.length}
                </Badge>
              )}
            </button>
          ))}
        </nav>
      </div>

      {/* Tab Content */}
      {activeTab === 'metrics' && (
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
                    <CartesianGrid strokeDasharray="3 3" />
                    <XAxis dataKey="timestamp" tickFormatter={formatTime} />
                    <YAxis unit="ms" />
                    <Tooltip
                      labelFormatter={(label) => new Date(label).toLocaleString()}
                      formatter={(value: number) => [`${value.toFixed(2)}ms`, 'Latency']}
                    />
                    <Area
                      type="monotone"
                      dataKey="average_latency_ms"
                      stroke="#3b82f6"
                      fill="#93c5fd"
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
              <CardDescription>Percentage of failed requests over {timeRange}</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="h-80">
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={history?.data_points || []}>
                    <CartesianGrid strokeDasharray="3 3" />
                    <XAxis dataKey="timestamp" tickFormatter={formatTime} />
                    <YAxis unit="%" domain={[0, 'auto']} />
                    <Tooltip
                      labelFormatter={(label) => new Date(label).toLocaleString()}
                      formatter={(value: number) => [`${(value * 100).toFixed(2)}%`, 'Error Rate']}
                    />
                    <Line
                      type="monotone"
                      dataKey="error_rate"
                      stroke="#ef4444"
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
                  <div className="flex justify-between items-center">
                    <span className="text-sm text-muted-foreground">Connections Open</span>
                    <span className="font-mono font-medium">{snapshot?.database.connections_open || 0}</span>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-sm text-muted-foreground">Connections In Use</span>
                    <span className="font-mono font-medium">{snapshot?.database.connections_in_use || 0}</span>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-sm text-muted-foreground">Avg Query Time</span>
                    <span className="font-mono font-medium">{((snapshot?.database.average_query_time || 0) * 1000).toFixed(2)}ms</span>
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
                  <div className="flex justify-between items-center">
                    <span className="text-sm text-muted-foreground">Hit Rate</span>
                    <span className={`font-mono font-medium ${(snapshot?.cache.hit_rate || 0) < 0.8 ? 'text-yellow-600' : 'text-green-600'}`}>
                      {((snapshot?.cache.hit_rate || 0) * 100).toFixed(1)}%
                    </span>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-sm text-muted-foreground">Avg Latency</span>
                    <span className="font-mono font-medium">{((snapshot?.cache.average_latency || 0) * 1000).toFixed(2)}ms</span>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>
        </div>
      )}

      {activeTab === 'health' && (
        <Card>
          <CardHeader>
            <CardTitle>Service Health</CardTitle>
            <CardDescription>
              {serviceHealth?.healthy_count} healthy, {serviceHealth?.degraded_count} degraded, {serviceHealth?.unhealthy_count} unhealthy
            </CardDescription>
          </CardHeader>
          <CardContent>
            {serviceHealth?.services.length === 0 ? (
              <div className="text-center py-12 text-muted-foreground">
                <svg className="w-12 h-12 mx-auto mb-4 text-gray-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2" />
                </svg>
                <p>No services found</p>
              </div>
            ) : (
              <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
                {serviceHealth?.services.map((service) => (
                  <div
                    key={service.service_id}
                    className={`p-4 border rounded-lg ${
                      service.status === 'healthy' ? 'border-green-200 bg-green-50' :
                      service.status === 'degraded' ? 'border-yellow-200 bg-yellow-50' :
                      service.status === 'unhealthy' ? 'border-red-200 bg-red-50' :
                      'border-gray-200'
                    }`}
                  >
                    <div className="flex items-center justify-between mb-2">
                      <span className="font-medium">{service.service_name}</span>
                      <Badge className={getStatusColor(service.status)}>
                        {service.status}
                      </Badge>
                    </div>
                    {service.project_slug && (
                      <p className="text-xs text-muted-foreground mb-2">{service.project_slug}</p>
                    )}
                    <div className="grid grid-cols-2 gap-2 text-sm">
                      <div>
                        <span className="text-muted-foreground">Uptime:</span>
                        <span className="ml-1 font-mono">{service.uptime.toFixed(1)}%</span>
                      </div>
                      <div>
                        <span className="text-muted-foreground">Pods:</span>
                        <span className="ml-1 font-mono">{service.ready_pods}/{service.pod_count}</span>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {activeTab === 'errors' && (
        <Card>
          <CardHeader>
            <CardTitle>Recent Errors</CardTitle>
            <CardDescription>{errors?.total_count || 0} errors in the last {errors?.time_range}</CardDescription>
          </CardHeader>
          <CardContent>
            {errors?.errors.length === 0 ? (
              <div className="text-center py-12 text-muted-foreground">
                <svg className="w-12 h-12 mx-auto mb-4 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <p className="font-medium">No errors detected</p>
                <p className="text-sm mt-1">Your services are running smoothly</p>
              </div>
            ) : (
              <div className="space-y-4">
                {errors?.errors.map((err) => (
                  <div key={err.id} className="p-4 border border-red-200 rounded-lg bg-red-50">
                    <div className="flex items-start justify-between">
                      <div className="flex-1">
                        <div className="flex items-center gap-2 mb-1">
                          <Badge variant="destructive">{err.level}</Badge>
                          {err.service_name && (
                            <span className="text-sm text-muted-foreground">{err.service_name}</span>
                          )}
                        </div>
                        <p className="font-mono text-sm">{err.message}</p>
                        {err.stack_trace && (
                          <pre className="mt-2 p-2 bg-gray-900 text-gray-100 text-xs rounded overflow-x-auto">
                            {err.stack_trace}
                          </pre>
                        )}
                      </div>
                      <div className="text-right text-xs text-muted-foreground ml-4">
                        <div>{new Date(err.timestamp).toLocaleString()}</div>
                        {err.count > 1 && (
                          <div className="mt-1">{err.count} occurrences</div>
                        )}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {activeTab === 'alerts' && (
        <Card>
          <CardHeader>
            <CardTitle>Active Alerts</CardTitle>
            <CardDescription>
              {alerts?.critical_count} critical, {alerts?.warning_count} warning, {alerts?.info_count} info
            </CardDescription>
          </CardHeader>
          <CardContent>
            {alerts?.alerts.length === 0 ? (
              <div className="text-center py-12 text-muted-foreground">
                <svg className="w-12 h-12 mx-auto mb-4 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <p className="font-medium">No active alerts</p>
                <p className="text-sm mt-1">All systems operating normally</p>
              </div>
            ) : (
              <div className="space-y-4">
                {alerts?.alerts.map((alert) => (
                  <div
                    key={alert.id}
                    className={`p-4 border rounded-lg ${getSeverityColor(alert.severity)}`}
                  >
                    <div className="flex items-start justify-between">
                      <div className="flex-1">
                        <div className="flex items-center gap-2 mb-1">
                          <Badge variant={alert.severity === 'critical' ? 'destructive' : 'secondary'}>
                            {alert.severity}
                          </Badge>
                          <span className="font-medium">{alert.name}</span>
                        </div>
                        <p className="text-sm text-muted-foreground">{alert.message}</p>
                        {alert.value !== undefined && alert.threshold !== undefined && (
                          <p className="text-sm mt-1">
                            <span className="font-mono">Current: {alert.value.toFixed(2)}</span>
                            <span className="mx-2">|</span>
                            <span className="font-mono">Threshold: {alert.threshold.toFixed(2)}</span>
                          </p>
                        )}
                        {alert.service_name && (
                          <p className="text-xs text-muted-foreground mt-1">Service: {alert.service_name}</p>
                        )}
                      </div>
                      <div className="text-right text-xs text-muted-foreground ml-4">
                        <div>Fired {new Date(alert.fired_at).toLocaleTimeString()}</div>
                        <Badge variant="outline" className="mt-1">{alert.status}</Badge>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
