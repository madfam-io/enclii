'use client';

import { useState, useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { apiGet } from "@/lib/api";
import { NetworkingTab } from "@/components/networking";
import { EnvVarsTab } from "@/components/env-vars";
import { PreviewsTab } from "@/components/previews";
import { SettingsTab } from "@/components/settings";
import { LogsTab } from "@/components/log-viewer";

interface ServiceDetail {
  id: string;
  name: string;
  project_id: string;
  project_name: string;
  project_slug?: string;
  environment: string;
  status: "healthy" | "unhealthy" | "unknown";
  version: string;
  replicas: string;
  created_at?: string;
  updated_at?: string;
  config?: {
    image?: string;
    port?: number;
    cpu_limit?: string;
    memory_limit?: string;
    env_vars?: Record<string, string>;
  };
  metrics?: {
    cpu_usage?: string;
    memory_usage?: string;
    request_count?: number;
    error_rate?: string;
  };
}

export default function ServiceDetailPage() {
  const params = useParams();
  const router = useRouter();
  const serviceId = params.id as string;

  const [service, setService] = useState<ServiceDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchService = async () => {
    try {
      setError(null);
      const data = await apiGet<ServiceDetail>(`/v1/services/${serviceId}`);
      setService(data);
      setLoading(false);
    } catch (err) {
      console.error("Failed to fetch service:", err);
      const message = err instanceof Error ? err.message : "Failed to fetch service details";
      // Handle specific error cases
      if (message.includes("not found") || message.includes("404")) {
        setError("Service not found");
      } else {
        setError(message);
      }
      setLoading(false);
    }
  };

  useEffect(() => {
    if (serviceId) {
      fetchService();
    }
  }, [serviceId]);

  const getStatusBadgeClass = (status: string) => {
    switch (status) {
      case "healthy":
        return "bg-green-100 text-green-800";
      case "unhealthy":
        return "bg-red-100 text-red-800";
      default:
        return "bg-gray-100 text-gray-800";
    }
  };

  const getEnvironmentBadgeClass = (env: string) => {
    switch (env) {
      case "production":
        return "bg-green-100 text-green-800";
      case "staging":
        return "bg-yellow-100 text-yellow-800";
      default:
        return "bg-blue-100 text-blue-800";
    }
  };

  if (loading) {
    return (
      <div className="container mx-auto py-8">
        <div className="mb-6">
          <Link href="/services" className="text-blue-600 hover:text-blue-800 flex items-center gap-1">
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
            </svg>
            Back to Services
          </Link>
        </div>
        <Card>
          <CardContent className="py-12">
            <div className="flex items-center justify-center">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
              <span className="ml-3 text-muted-foreground">Loading service details...</span>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  if (error) {
    return (
      <div className="container mx-auto py-8">
        <div className="mb-6">
          <Link href="/services" className="text-blue-600 hover:text-blue-800 flex items-center gap-1">
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
            </svg>
            Back to Services
          </Link>
        </div>
        <Card className="border-red-200 bg-red-50">
          <CardContent className="py-8">
            <div className="text-center">
              <p className="text-red-600 font-medium mb-4">{error}</p>
              <div className="space-x-4">
                <button
                  onClick={fetchService}
                  className="inline-flex items-center px-4 py-2 border border-red-300 rounded-md shadow-sm text-sm font-medium text-red-700 bg-white hover:bg-red-50"
                >
                  Try Again
                </button>
                <button
                  onClick={() => router.push("/services")}
                  className="inline-flex items-center px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
                >
                  Go to Services
                </button>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  if (!service) {
    return null;
  }

  return (
    <div className="container mx-auto py-8">
      {/* Breadcrumb */}
      <div className="mb-6">
        <Link href="/services" className="text-blue-600 hover:text-blue-800 flex items-center gap-1">
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
          </svg>
          Back to Services
        </Link>
      </div>

      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-3xl font-bold">{service.name}</h1>
          <p className="text-muted-foreground mt-2">
            Service details and configuration
          </p>
        </div>
        <div className="flex items-center gap-3">
          <span className={`inline-flex items-center px-3 py-1 rounded-full text-sm font-medium ${getEnvironmentBadgeClass(service.environment)}`}>
            {service.environment}
          </span>
          <span className={`inline-flex items-center px-3 py-1 rounded-full text-sm font-medium ${getStatusBadgeClass(service.status)}`}>
            {service.status}
          </span>
        </div>
      </div>

      {/* Tabs */}
      <Tabs defaultValue="overview" className="space-y-6">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="previews">Previews</TabsTrigger>
          <TabsTrigger value="env-vars">Environment</TabsTrigger>
          <TabsTrigger value="networking">Networking</TabsTrigger>
          <TabsTrigger value="deployments">Deployments</TabsTrigger>
          <TabsTrigger value="logs">Logs</TabsTrigger>
          <TabsTrigger value="settings">Settings</TabsTrigger>
        </TabsList>

        {/* Overview Tab */}
        <TabsContent value="overview">
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            {/* Overview Card */}
            <Card>
              <CardHeader>
                <CardTitle>Overview</CardTitle>
                <CardDescription>Basic service information</CardDescription>
              </CardHeader>
              <CardContent>
                <dl className="space-y-4">
                  <div className="flex justify-between">
                    <dt className="text-gray-500">Service ID</dt>
                    <dd className="font-mono text-sm">{service.id}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-gray-500">Project</dt>
                    <dd>
                      <Link
                        href={`/projects/${service.project_slug || service.project_name?.toLowerCase().replace(/\s+/g, '-')}`}
                        className="text-blue-600 hover:text-blue-800"
                      >
                        {service.project_name}
                      </Link>
                    </dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-gray-500">Version</dt>
                    <dd className="font-mono text-sm">{service.version || "N/A"}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-gray-500">Replicas</dt>
                    <dd>{service.replicas || "0/0"}</dd>
                  </div>
                  {service.created_at && (
                    <div className="flex justify-between">
                      <dt className="text-gray-500">Created</dt>
                      <dd>{new Date(service.created_at).toLocaleDateString()}</dd>
                    </div>
                  )}
                  {service.updated_at && (
                    <div className="flex justify-between">
                      <dt className="text-gray-500">Last Updated</dt>
                      <dd>{new Date(service.updated_at).toLocaleDateString()}</dd>
                    </div>
                  )}
                </dl>
              </CardContent>
            </Card>

            {/* Configuration Card */}
            <Card>
              <CardHeader>
                <CardTitle>Configuration</CardTitle>
                <CardDescription>Resource limits and settings</CardDescription>
              </CardHeader>
              <CardContent>
                <dl className="space-y-4">
                  {service.config?.image && (
                    <div className="flex justify-between">
                      <dt className="text-gray-500">Image</dt>
                      <dd className="font-mono text-sm truncate max-w-[200px]" title={service.config.image}>
                        {service.config.image}
                      </dd>
                    </div>
                  )}
                  {service.config?.port && (
                    <div className="flex justify-between">
                      <dt className="text-gray-500">Port</dt>
                      <dd>{service.config.port}</dd>
                    </div>
                  )}
                  {service.config?.cpu_limit && (
                    <div className="flex justify-between">
                      <dt className="text-gray-500">CPU Limit</dt>
                      <dd>{service.config.cpu_limit}</dd>
                    </div>
                  )}
                  {service.config?.memory_limit && (
                    <div className="flex justify-between">
                      <dt className="text-gray-500">Memory Limit</dt>
                      <dd>{service.config.memory_limit}</dd>
                    </div>
                  )}
                  {!service.config && (
                    <p className="text-gray-400 text-sm">No configuration data available</p>
                  )}
                </dl>
              </CardContent>
            </Card>

            {/* Metrics Card */}
            {service.metrics && (
              <Card>
                <CardHeader>
                  <CardTitle>Metrics</CardTitle>
                  <CardDescription>Current resource usage</CardDescription>
                </CardHeader>
                <CardContent>
                  <dl className="space-y-4">
                    {service.metrics.cpu_usage && (
                      <div className="flex justify-between">
                        <dt className="text-gray-500">CPU Usage</dt>
                        <dd>{service.metrics.cpu_usage}</dd>
                      </div>
                    )}
                    {service.metrics.memory_usage && (
                      <div className="flex justify-between">
                        <dt className="text-gray-500">Memory Usage</dt>
                        <dd>{service.metrics.memory_usage}</dd>
                      </div>
                    )}
                    {service.metrics.request_count !== undefined && (
                      <div className="flex justify-between">
                        <dt className="text-gray-500">Requests (24h)</dt>
                        <dd>{service.metrics.request_count.toLocaleString()}</dd>
                      </div>
                    )}
                    {service.metrics.error_rate && (
                      <div className="flex justify-between">
                        <dt className="text-gray-500">Error Rate</dt>
                        <dd>{service.metrics.error_rate}</dd>
                      </div>
                    )}
                  </dl>
                </CardContent>
              </Card>
            )}

            {/* Actions Card */}
            <Card>
              <CardHeader>
                <CardTitle>Actions</CardTitle>
                <CardDescription>Service operations</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="space-y-3">
                  <button
                    className="w-full inline-flex items-center justify-center px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
                    onClick={() => router.push(`/deployments?service=${serviceId}`)}
                  >
                    <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
                    </svg>
                    View Deployments
                  </button>
                  <button
                    className="w-full inline-flex items-center justify-center px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
                    onClick={() => {
                      // Switch to logs tab
                      const logsTab = document.querySelector('[data-state="inactive"][value="logs"]') as HTMLElement;
                      if (logsTab) logsTab.click();
                    }}
                  >
                    <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                    </svg>
                    View Logs
                  </button>
                  <button
                    className="w-full inline-flex items-center justify-center px-4 py-2 border border-blue-300 rounded-md shadow-sm text-sm font-medium text-blue-700 bg-blue-50 hover:bg-blue-100"
                    onClick={fetchService}
                  >
                    <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                    </svg>
                    Refresh
                  </button>
                </div>
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        {/* Previews Tab */}
        <TabsContent value="previews">
          <PreviewsTab serviceId={serviceId} serviceName={service.name} />
        </TabsContent>

        {/* Environment Variables Tab */}
        <TabsContent value="env-vars">
          <EnvVarsTab serviceId={serviceId} serviceName={service.name} />
        </TabsContent>

        {/* Networking Tab */}
        <TabsContent value="networking">
          <NetworkingTab serviceId={serviceId} serviceName={service.name} />
        </TabsContent>

        {/* Deployments Tab */}
        <TabsContent value="deployments">
          <Card>
            <CardHeader>
              <CardTitle>Deployments</CardTitle>
              <CardDescription>Recent deployment history for this service</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="text-center py-8 text-muted-foreground">
                <svg className="w-12 h-12 mx-auto mb-4 text-gray-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
                </svg>
                <p>Deployment history coming soon</p>
                <button
                  onClick={() => router.push(`/deployments?service=${serviceId}`)}
                  className="mt-4 text-blue-600 hover:text-blue-800"
                >
                  View all deployments →
                </button>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        {/* Logs Tab */}
        <TabsContent value="logs">
          <LogsTab serviceId={serviceId} serviceName={service.name} env={service.environment} />
        </TabsContent>

        {/* Settings Tab */}
        <TabsContent value="settings">
          <SettingsTab serviceId={serviceId} serviceName={service.name} />
        </TabsContent>
      </Tabs>
    </div>
  );
}
