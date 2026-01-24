'use client'

import { cn } from '@/lib/utils'
import { formatResponseTime } from '@/lib/utils'
import { getResponseTimeStatus } from '@/lib/types'

interface ResponseTimeProps {
  ms: number | null
  showLabel?: boolean
  size?: 'sm' | 'md'
}

const statusColors = {
  fast: 'text-status-operational',
  normal: 'text-status-degraded',
  slow: 'text-status-degraded',
  critical: 'text-status-outage',
  unknown: 'text-muted-foreground',
}

const statusLabels = {
  fast: 'Fast',
  normal: 'Normal',
  slow: 'Slow',
  critical: 'Very Slow',
  unknown: 'Unknown',
}

export function ResponseTime({ ms, showLabel = false, size = 'md' }: ResponseTimeProps) {
  const status = getResponseTimeStatus(ms)
  const formatted = formatResponseTime(ms)

  return (
    <div
      className={cn(
        'inline-flex items-center gap-1.5 font-mono',
        size === 'sm' ? 'text-xs' : 'text-sm',
        statusColors[status]
      )}
    >
      <span>{formatted}</span>
      {showLabel && ms !== null && (
        <span className="text-muted-foreground">({statusLabels[status]})</span>
      )}
    </div>
  )
}

interface ResponseTimeBarProps {
  ms: number | null
  maxMs?: number
}

export function ResponseTimeBar({ ms, maxMs = 1000 }: ResponseTimeBarProps) {
  const status = getResponseTimeStatus(ms)
  const percentage = ms ? Math.min((ms / maxMs) * 100, 100) : 0

  const barColors = {
    fast: 'bg-status-operational',
    normal: 'bg-status-degraded',
    slow: 'bg-status-degraded',
    critical: 'bg-status-outage',
    unknown: 'bg-muted',
  }

  return (
    <div className="flex items-center gap-2 w-full">
      <div className="flex-1 h-1.5 bg-muted rounded-full overflow-hidden">
        <div
          className={cn('h-full rounded-full transition-all duration-300', barColors[status])}
          style={{ width: `${percentage}%` }}
        />
      </div>
      <span className={cn('text-xs font-mono min-w-[60px] text-right', statusColors[status])}>
        {formatResponseTime(ms)}
      </span>
    </div>
  )
}
