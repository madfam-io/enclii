'use client';

import { useState, useEffect, useCallback } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
  Legend,
} from 'recharts';
import { apiGet } from '@/lib/api';

interface UsageMetric {
  type: string;
  label: string;
  used: number;
  included: number;
  unit: string;
  cost: number;
}

interface UsageSummary {
  period_start: string;
  period_end: string;
  metrics: UsageMetric[];
  total_cost: number;
  plan_base: number;
  grand_total: number;
  plan_name: string;
}

interface CostCategory {
  name: string;
  value: number;
  color: string;
}

interface CostBreakdown {
  period_start: string;
  period_end: string;
  plan_base: number;
  plan_name: string;
  categories: CostCategory[];
  total_usage: number;
  grand_total: number;
}

function getProgressColor(percentage: number): string {
  if (percentage >= 90) return 'bg-red-500';
  if (percentage >= 75) return 'bg-yellow-500';
  return 'bg-green-500';
}

function formatNumber(num: number): string {
  if (num >= 1000) {
    return (num / 1000).toFixed(1) + 'k';
  }
  return num.toFixed(1);
}

const iconMap: Record<string, React.ReactNode> = {
  compute: (
    <svg className="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" />
    </svg>
  ),
  build: (
    <svg className="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
    </svg>
  ),
  storage: (
    <svg className="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4" />
    </svg>
  ),
  bandwidth: (
    <svg className="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
    </svg>
  ),
  domains: (
    <svg className="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9" />
    </svg>
  ),
};

export default function UsagePage() {
  const [usage, setUsage] = useState<UsageSummary | null>(null);
  const [costs, setCosts] = useState<CostBreakdown | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    try {
      setError(null);
      setLoading(true);

      const [usageData, costsData] = await Promise.all([
        apiGet<UsageSummary>('/v1/usage'),
        apiGet<CostBreakdown>('/v1/usage/costs'),
      ]);

      setUsage(usageData);
      setCosts(costsData);
    } catch (err) {
      console.error('Failed to fetch usage data:', err);
      setError(err instanceof Error ? err.message : 'Failed to fetch usage data');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-24">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
        <span className="ml-3 text-muted-foreground">Loading usage data...</span>
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

  // Prepare chart data
  const usageChartData = usage?.metrics.map(m => ({
    name: m.label,
    used: m.used,
    included: m.included === -1 ? m.used : m.included,
    percentage: m.included === -1 ? 0 : Math.min((m.used / m.included) * 100, 150),
  })) || [];

  const costChartData = costs?.categories.filter(c => c.value > 0) || [];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Usage Analytics</h1>
          <p className="text-muted-foreground">
            Monitor your resource usage and billing for the current period
          </p>
        </div>
        <Button variant="outline" onClick={fetchData}>
          <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
          Refresh
        </Button>
      </div>

      {/* Billing Period & Summary */}
      <div className="grid gap-4 md:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Current Period</CardTitle>
            <svg className="w-4 h-4 text-muted-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
            </svg>
          </CardHeader>
          <CardContent>
            <div className="text-lg font-bold">
              {usage?.period_start} - {usage?.period_end}
            </div>
            <p className="text-xs text-muted-foreground">{usage?.plan_name} Plan</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Plan Base</CardTitle>
            <svg className="w-4 h-4 text-muted-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 7h6m0 10v-3m-3 3h.01M9 17h.01M9 14h.01M12 14h.01M15 11h.01M12 11h.01M9 11h.01M7 21h10a2 2 0 002-2V5a2 2 0 00-2-2H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
            </svg>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">${usage?.plan_base.toFixed(2)}</div>
            <p className="text-xs text-muted-foreground">Monthly subscription</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Usage Charges</CardTitle>
            <svg className="w-4 h-4 text-muted-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-blue-600">${usage?.total_cost.toFixed(2)}</div>
            <p className="text-xs text-muted-foreground">Overage this period</p>
          </CardContent>
        </Card>
        <Card className="bg-gradient-to-br from-blue-50 to-blue-100 dark:from-blue-950 dark:to-blue-900">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Estimated Total</CardTitle>
            <svg className="w-4 h-4 text-blue-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 9V7a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2m2 4h10a2 2 0 002-2v-6a2 2 0 00-2-2H9a2 2 0 00-2 2v6a2 2 0 002 2zm7-5a2 2 0 11-4 0 2 2 0 014 0z" />
            </svg>
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold text-blue-700">${usage?.grand_total.toFixed(2)}</div>
            <p className="text-xs text-muted-foreground">Due at period end</p>
          </CardContent>
        </Card>
      </div>

      {/* Usage Meters & Cost Breakdown */}
      <div className="grid gap-6 md:grid-cols-2">
        {/* Usage Meters */}
        <Card>
          <CardHeader>
            <CardTitle>Resource Usage</CardTitle>
            <CardDescription>Current usage against included allocations</CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            {usage?.metrics.map((metric) => {
              const isUnlimited = metric.included === -1;
              const percentage = isUnlimited
                ? 0
                : Math.min((metric.used / metric.included) * 100, 100);
              const overLimit = !isUnlimited && metric.used > metric.included;

              return (
                <div key={metric.type} className="space-y-2">
                  <div className="flex items-center justify-between text-sm">
                    <div className="flex items-center gap-2">
                      <span className="text-muted-foreground">{iconMap[metric.type]}</span>
                      <span className="font-medium">{metric.label}</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className={`font-mono ${overLimit ? 'text-red-500' : ''}`}>
                        {formatNumber(metric.used)}
                      </span>
                      <span className="text-muted-foreground">/</span>
                      <span className="text-muted-foreground font-mono">
                        {isUnlimited ? '∞' : formatNumber(metric.included)}
                      </span>
                      <span className="text-muted-foreground text-xs">{metric.unit}</span>
                    </div>
                  </div>

                  {!isUnlimited && (
                    <div className="relative">
                      <Progress value={percentage} className="h-2" />
                      <div
                        className={`absolute top-0 left-0 h-2 rounded-full transition-all ${getProgressColor(percentage)}`}
                        style={{ width: `${Math.min(percentage, 100)}%` }}
                      />
                      {overLimit && (
                        <div
                          className="absolute top-0 h-2 bg-red-500/30 rounded-r-full"
                          style={{
                            left: '100%',
                            width: `${((metric.used - metric.included) / metric.included) * 100}%`,
                            maxWidth: '20%',
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

        {/* Cost Breakdown Pie Chart */}
        <Card>
          <CardHeader>
            <CardTitle>Cost Breakdown</CardTitle>
            <CardDescription>Usage costs by category</CardDescription>
          </CardHeader>
          <CardContent>
            {costChartData.length > 0 ? (
              <div className="h-64">
                <ResponsiveContainer width="100%" height="100%">
                  <PieChart>
                    <Pie
                      data={costChartData}
                      cx="50%"
                      cy="50%"
                      innerRadius={60}
                      outerRadius={80}
                      paddingAngle={2}
                      dataKey="value"
                      label={({ name, value }) => `${name}: $${value.toFixed(2)}`}
                    >
                      {costChartData.map((entry, index) => (
                        <Cell key={`cell-${index}`} fill={entry.color} />
                      ))}
                    </Pie>
                    <Tooltip formatter={(value: number) => [`$${value.toFixed(2)}`, '']} />
                    <Legend />
                  </PieChart>
                </ResponsiveContainer>
              </div>
            ) : (
              <div className="flex items-center justify-center h-64 text-muted-foreground">
                <div className="text-center">
                  <svg className="w-12 h-12 mx-auto mb-4 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                  <p className="font-medium">No overage charges</p>
                  <p className="text-sm">You&apos;re within your plan limits</p>
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Usage Bar Chart */}
      <Card>
        <CardHeader>
          <CardTitle>Usage Overview</CardTitle>
          <CardDescription>Comparing used resources against included allocations</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="h-80">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={usageChartData} margin={{ top: 20, right: 30, left: 20, bottom: 5 }}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="name" />
                <YAxis />
                <Tooltip
                  formatter={(value: number, name: string) => [
                    name === 'percentage' ? `${value.toFixed(1)}%` : value.toFixed(1),
                    name === 'used' ? 'Used' : name === 'included' ? 'Included' : 'Usage %',
                  ]}
                />
                <Legend />
                <Bar dataKey="used" fill="#3b82f6" name="Used" />
                <Bar dataKey="included" fill="#e5e7eb" name="Included" />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </CardContent>
      </Card>

      {/* Cost Details Table */}
      <Card>
        <CardHeader>
          <CardTitle>Detailed Cost Summary</CardTitle>
          <CardDescription>Breakdown of all charges for the current billing period</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="border-b">
                  <th className="text-left py-3 px-4 font-medium text-muted-foreground">Category</th>
                  <th className="text-right py-3 px-4 font-medium text-muted-foreground">Used</th>
                  <th className="text-right py-3 px-4 font-medium text-muted-foreground">Included</th>
                  <th className="text-right py-3 px-4 font-medium text-muted-foreground">Overage</th>
                  <th className="text-right py-3 px-4 font-medium text-muted-foreground">Cost</th>
                </tr>
              </thead>
              <tbody>
                {usage?.metrics.map((metric) => {
                  const overage = metric.included === -1 ? 0 : Math.max(0, metric.used - metric.included);
                  return (
                    <tr key={metric.type} className="border-b">
                      <td className="py-3 px-4">
                        <div className="flex items-center gap-2">
                          <span className="text-muted-foreground">{iconMap[metric.type]}</span>
                          <span>{metric.label}</span>
                        </div>
                      </td>
                      <td className="text-right py-3 px-4 font-mono">
                        {formatNumber(metric.used)} {metric.unit}
                      </td>
                      <td className="text-right py-3 px-4 font-mono text-muted-foreground">
                        {metric.included === -1 ? 'Unlimited' : `${formatNumber(metric.included)} ${metric.unit}`}
                      </td>
                      <td className="text-right py-3 px-4 font-mono">
                        {overage > 0 ? (
                          <span className="text-red-500">+{formatNumber(overage)} {metric.unit}</span>
                        ) : (
                          <span className="text-green-500">-</span>
                        )}
                      </td>
                      <td className="text-right py-3 px-4 font-mono font-medium">
                        {metric.cost > 0 ? (
                          <span className="text-red-500">${metric.cost.toFixed(2)}</span>
                        ) : (
                          <span className="text-green-500">$0.00</span>
                        )}
                      </td>
                    </tr>
                  );
                })}
                <tr className="bg-gray-50">
                  <td colSpan={4} className="py-3 px-4 font-medium">Plan Base ({usage?.plan_name})</td>
                  <td className="text-right py-3 px-4 font-mono font-medium">${usage?.plan_base.toFixed(2)}</td>
                </tr>
                <tr className="bg-blue-50">
                  <td colSpan={4} className="py-3 px-4 font-bold">Estimated Total</td>
                  <td className="text-right py-3 px-4 font-mono font-bold text-blue-600">${usage?.grand_total.toFixed(2)}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>

      {/* Footer Note */}
      <p className="text-sm text-muted-foreground text-center">
        Final invoice will be generated on the 1st of each month. Usage estimates are updated hourly.
      </p>
    </div>
  );
}
