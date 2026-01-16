// Hooks index - Custom React hooks for UI functionality
export { useScrollShadow, useIsScrolled } from './use-scroll-shadow';
export {
  useUsageMetrics,
  useMetricByType,
  useComputeUsage,
  useBuildUsage,
  useStorageUsage,
  useBandwidthUsage,
  useRealtimeResources,
  formatBytes,
  formatNumber,
  getUsageColor,
} from './use-usage-metrics';
export type {
  UsageMetric,
  UsageSummary,
  RealtimeMetrics,
  ServiceMetrics,
} from './use-usage-metrics';
