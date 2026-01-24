/**
 * Format a date string for display.
 *
 * @param dateString - ISO date string or null
 * @returns Formatted date string or "Never" if null
 *
 * @example
 * formatDate("2025-01-15T10:30:00Z") // => "Jan 15, 2025, 10:30 AM"
 * formatDate(null) // => "Never"
 */
export function formatDate(dateString: string | null): string {
  if (!dateString) return "Never";
  const date = new Date(dateString);
  return new Intl.DateTimeFormat("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

/**
 * Format relative time (e.g., "2 hours ago").
 *
 * @param dateString - ISO date string
 * @returns Relative time string
 *
 * @example
 * formatRelativeTime("2025-01-15T10:30:00Z") // => "2h ago" (if 2 hours ago)
 */
export function formatRelativeTime(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diffInSeconds = Math.floor((now.getTime() - date.getTime()) / 1000);

  if (diffInSeconds < 0) return "just now"; // Future dates
  if (diffInSeconds < 60) return "just now";
  if (diffInSeconds < 3600) return `${Math.floor(diffInSeconds / 60)}m ago`;
  if (diffInSeconds < 86400) return `${Math.floor(diffInSeconds / 3600)}h ago`;
  if (diffInSeconds < 604800)
    return `${Math.floor(diffInSeconds / 86400)}d ago`;
  return formatDate(dateString);
}

/**
 * Format a number with compact notation (e.g., 1.2K, 3.4M).
 *
 * @param num - Number to format
 * @returns Formatted string with compact notation
 *
 * @example
 * formatCompact(1234) // => "1.2K"
 * formatCompact(1234567) // => "1.2M"
 */
export function formatCompact(num: number): string {
  return new Intl.NumberFormat("en-US", {
    notation: "compact",
    compactDisplay: "short",
    maximumFractionDigits: 1,
  }).format(num);
}

/**
 * Format bytes to human-readable string.
 *
 * @param bytes - Number of bytes
 * @param decimals - Number of decimal places (default: 2)
 * @returns Formatted string (e.g., "1.5 GB")
 */
export function formatBytes(bytes: number, decimals: number = 2): string {
  if (bytes === 0) return "0 Bytes";

  const k = 1024;
  const sizes = ["Bytes", "KB", "MB", "GB", "TB", "PB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));

  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(decimals))} ${sizes[i]}`;
}
