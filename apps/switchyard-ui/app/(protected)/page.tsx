"use client";

import { useState, useEffect } from "react";
import Link from "next/link";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:4200";

interface DashboardStats {
  healthy_services: number;
  deployments_today: number;
  active_projects: number;
  avg_deploy_time: string;
}

interface RecentActivity {
  id: string;
  type: string;
  message: string;
  timestamp: string;
  status: "success" | "running" | "failed" | "pending";
  metadata?: any;
}

interface ServiceOverview {
  id: string;
  name: string;
  project_name: string;
  environment: string;
  status: "healthy" | "unhealthy" | "unknown";
  version: string;
  replicas: string;
}

interface DashboardResponse {
  stats: DashboardStats;
  activities: RecentActivity[];
  services: ServiceOverview[];
}

export default function Dashboard() {
  const [stats, setStats] = useState<DashboardStats>({
    healthy_services: 0,
    deployments_today: 0,
    active_projects: 0,
    avg_deploy_time: "N/A",
  });
  const [activities, setActivities] = useState<RecentActivity[]>([]);
  const [services, setServices] = useState<ServiceOverview[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchDashboardData = async () => {
    try {
      setError(null);
      const response = await fetch(`${API_BASE_URL}/v1/dashboard/stats`);

      if (!response.ok) {
        throw new Error(`API error: ${response.status} ${response.statusText}`);
      }

      const data: DashboardResponse = await response.json();

      setStats(data.stats);
      setActivities(data.activities || []);
      setServices(data.services || []);
      setLoading(false);
    } catch (err) {
      console.error("Failed to fetch dashboard data:", err);
      setError(
        err instanceof Error ? err.message : "Failed to fetch dashboard data",
      );
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchDashboardData();

    // Refresh data every 30 seconds
    const interval = setInterval(fetchDashboardData, 30000);
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

  if (loading) {
    return (
      <div className="max-w-7xl mx-auto py-6 sm:px-6 lg:px-8">
        <div className="animate-pulse">
          <div className="h-8 bg-gray-200 rounded w-1/4 mb-8"></div>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
            {[1, 2, 3, 4].map((i) => (
              <div key={i} className="h-24 bg-gray-200 rounded"></div>
            ))}
          </div>
          <div className="space-y-6">
            <div className="h-64 bg-gray-200 rounded"></div>
            <div className="h-64 bg-gray-200 rounded"></div>
          </div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="max-w-7xl mx-auto py-6 sm:px-6 lg:px-8">
        <div className="bg-red-50 border border-red-200 rounded-lg p-6">
          <h2 className="text-lg font-semibold text-red-800 mb-2">
            Error Loading Dashboard
          </h2>
          <p className="text-red-600 mb-4">{error}</p>
          <button
            onClick={fetchDashboardData}
            className="inline-flex items-center px-4 py-2 border border-red-300 rounded-md shadow-sm text-sm font-medium text-red-700 bg-white hover:bg-red-50"
          >
            Try Again
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto py-6 sm:px-6 lg:px-8">
      <div className="px-4 py-6 sm:px-0">
        <div className="flex items-center justify-between mb-8">
          <h1 className="text-3xl font-bold text-gray-900">Dashboard</h1>
          <button
            onClick={fetchDashboardData}
            className="inline-flex items-center px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
          >
            <svg
              className="w-4 h-4 mr-2"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
              />
            </svg>
            Refresh
          </button>
        </div>

        {/* Status Cards */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
          <div className="bg-white overflow-hidden shadow rounded-lg">
            <div className="p-5">
              <div className="flex items-center">
                <div className="flex-shrink-0">
                  <div className="w-8 h-8 bg-enclii-green rounded-full flex items-center justify-center">
                    <span className="text-white text-sm font-bold">✓</span>
                  </div>
                </div>
                <div className="ml-5 w-0 flex-1">
                  <dl>
                    <dt className="text-sm font-medium text-gray-500 truncate">
                      Healthy Services
                    </dt>
                    <dd className="text-lg font-medium text-gray-900">
                      {stats.healthy_services}
                    </dd>
                  </dl>
                </div>
              </div>
            </div>
          </div>

          <div className="bg-white overflow-hidden shadow rounded-lg">
            <div className="p-5">
              <div className="flex items-center">
                <div className="flex-shrink-0">
                  <div className="w-8 h-8 bg-enclii-orange rounded-full flex items-center justify-center">
                    <span className="text-white text-sm font-bold">⚠</span>
                  </div>
                </div>
                <div className="ml-5 w-0 flex-1">
                  <dl>
                    <dt className="text-sm font-medium text-gray-500 truncate">
                      Deployments Today
                    </dt>
                    <dd className="text-lg font-medium text-gray-900">
                      {stats.deployments_today}
                    </dd>
                  </dl>
                </div>
              </div>
            </div>
          </div>

          <div className="bg-white overflow-hidden shadow rounded-lg">
            <div className="p-5">
              <div className="flex items-center">
                <div className="flex-shrink-0">
                  <div className="w-8 h-8 bg-enclii-blue rounded-full flex items-center justify-center">
                    <span className="text-white text-sm font-bold">📊</span>
                  </div>
                </div>
                <div className="ml-5 w-0 flex-1">
                  <dl>
                    <dt className="text-sm font-medium text-gray-500 truncate">
                      Active Projects
                    </dt>
                    <dd className="text-lg font-medium text-gray-900">
                      {stats.active_projects}
                    </dd>
                  </dl>
                </div>
              </div>
            </div>
          </div>

          <div className="bg-white overflow-hidden shadow rounded-lg">
            <div className="p-5">
              <div className="flex items-center">
                <div className="flex-shrink-0">
                  <div className="w-8 h-8 bg-gray-400 rounded-full flex items-center justify-center">
                    <span className="text-white text-sm font-bold">🕐</span>
                  </div>
                </div>
                <div className="ml-5 w-0 flex-1">
                  <dl>
                    <dt className="text-sm font-medium text-gray-500 truncate">
                      Avg Deploy Time
                    </dt>
                    <dd className="text-lg font-medium text-gray-900">
                      {stats.avg_deploy_time}
                    </dd>
                  </dl>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Recent Activity */}
        <div className="bg-white shadow overflow-hidden sm:rounded-md mb-8">
          <div className="px-4 py-5 sm:px-6">
            <h3 className="text-lg leading-6 font-medium text-gray-900">
              Recent Activity
            </h3>
            <p className="mt-1 max-w-2xl text-sm text-gray-500">
              Latest deployments and system events
            </p>
          </div>
          <ul className="divide-y divide-gray-200">
            {activities.length === 0 ? (
              <li className="px-4 py-8 text-center text-gray-500">
                No recent activity
              </li>
            ) : (
              activities.map((activity) => (
                <li key={activity.id} className="px-4 py-4">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center">
                      <div
                        className={`w-2 h-2 rounded-full mr-3 ${
                          activity.status === "success"
                            ? "bg-green-500"
                            : activity.status === "running"
                              ? "bg-blue-500"
                              : activity.status === "failed"
                                ? "bg-red-500"
                                : "bg-yellow-500"
                        }`}
                      ></div>
                      <div>
                        <p className="text-sm font-medium text-gray-900">
                          {activity.message}
                        </p>
                        <p className="text-sm text-gray-500">
                          {activity.metadata?.version ||
                            activity.metadata?.environment ||
                            ""}{" "}
                          • {formatTimeAgo(activity.timestamp)}
                        </p>
                      </div>
                    </div>
                    <span
                      className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                        activity.status === "success"
                          ? "bg-green-100 text-green-800"
                          : activity.status === "running"
                            ? "bg-blue-100 text-blue-800"
                            : activity.status === "failed"
                              ? "bg-red-100 text-red-800"
                              : "bg-yellow-100 text-yellow-800"
                      }`}
                    >
                      {activity.status.charAt(0).toUpperCase() +
                        activity.status.slice(1)}
                    </span>
                  </div>
                </li>
              ))
            )}
          </ul>
        </div>

        {/* Services Overview */}
        <div className="bg-white shadow overflow-hidden sm:rounded-md">
          <div className="px-4 py-5 sm:px-6">
            <h3 className="text-lg leading-6 font-medium text-gray-900">
              Services Overview
            </h3>
            <p className="mt-1 max-w-2xl text-sm text-gray-500">
              Current status of all services
            </p>
          </div>
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Service
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Environment
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Status
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Version
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Replicas
                  </th>
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-gray-200">
                {services.length === 0 ? (
                  <tr>
                    <td
                      colSpan={5}
                      className="px-6 py-8 text-center text-gray-500"
                    >
                      No services found
                    </td>
                  </tr>
                ) : (
                  services.map((service) => (
                    <tr key={service.id} className="hover:bg-gray-50">
                      <td className="px-6 py-4 whitespace-nowrap">
                        <div className="flex items-center">
                          <Link
                            href={`/services/${service.id}`}
                            className="text-sm font-medium text-gray-900 hover:text-enclii-blue"
                          >
                            {service.name}
                          </Link>
                          <div className="text-xs text-gray-500 ml-2">
                            in {service.project_name}
                          </div>
                        </div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <span
                          className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                            service.environment === "production"
                              ? "bg-green-100 text-green-800"
                              : service.environment === "staging"
                                ? "bg-yellow-100 text-yellow-800"
                                : "bg-blue-100 text-blue-800"
                          }`}
                        >
                          {service.environment}
                        </span>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <span
                          className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                            service.status === "healthy"
                              ? "bg-green-100 text-green-800"
                              : service.status === "unhealthy"
                                ? "bg-red-100 text-red-800"
                                : "bg-gray-100 text-gray-800"
                          }`}
                        >
                          {service.status}
                        </span>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                        {service.version}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                        {service.replicas}
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  );
}
