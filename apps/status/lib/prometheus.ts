import type { UptimeData, DayStatus, ServiceStatus } from './types'
import { getPrometheusUrl } from './config'

/**
 * Query Prometheus for uptime data
 */
async function queryPrometheus(query: string): Promise<unknown> {
  const baseUrl = getPrometheusUrl()
  if (!baseUrl) {
    throw new Error('Prometheus URL not configured')
  }

  const url = `${baseUrl}/api/v1/query?query=${encodeURIComponent(query)}`
  const response = await fetch(url)

  if (!response.ok) {
    throw new Error(`Prometheus query failed: ${response.status}`)
  }

  const data = await response.json()
  return data
}

/**
 * Query Prometheus for a range of data
 */
async function queryPrometheusRange(
  query: string,
  start: Date,
  end: Date,
  step: string
): Promise<unknown> {
  const baseUrl = getPrometheusUrl()
  if (!baseUrl) {
    throw new Error('Prometheus URL not configured')
  }

  const url = `${baseUrl}/api/v1/query_range?query=${encodeURIComponent(query)}&start=${start.toISOString()}&end=${end.toISOString()}&step=${step}`
  const response = await fetch(url)

  if (!response.ok) {
    throw new Error(`Prometheus range query failed: ${response.status}`)
  }

  const data = await response.json()
  return data
}

/**
 * Get uptime percentage for a service over a time period
 */
async function getUptimePercent(serviceUrl: string, duration: string): Promise<number | null> {
  try {
    // Using blackbox exporter probe_success metric
    const query = `avg_over_time(probe_success{instance="${serviceUrl}"}[${duration}]) * 100`
    const result = await queryPrometheus(query) as {
      data?: {
        result?: Array<{
          value?: [number, string]
        }>
      }
    }

    const value = result.data?.result?.[0]?.value?.[1]
    if (value !== undefined) {
      return parseFloat(value)
    }
    return null
  } catch (err) {
    console.error(`Failed to get uptime for ${serviceUrl}:`, err)
    return null
  }
}

/**
 * Get daily uptime history for the last 90 days
 */
async function getDailyHistory(serviceUrl: string): Promise<DayStatus[]> {
  const history: DayStatus[] = []

  try {
    const end = new Date()
    const start = new Date(end.getTime() - 90 * 24 * 60 * 60 * 1000)

    // Query for daily average uptime
    const query = `avg_over_time(probe_success{instance="${serviceUrl}"}[1d])`
    const result = await queryPrometheusRange(query, start, end, '1d') as {
      data?: {
        result?: Array<{
          values?: Array<[number, string]>
        }>
      }
    }

    const values = result.data?.result?.[0]?.values || []

    for (const [timestamp, value] of values) {
      const uptimePercent = parseFloat(value) * 100
      const date = new Date(timestamp * 1000).toISOString().split('T')[0]

      let status: ServiceStatus = 'operational'
      if (uptimePercent < 99) status = 'degraded'
      if (uptimePercent < 95) status = 'outage'

      history.push({
        date,
        status,
        uptimePercent,
      })
    }
  } catch (err) {
    console.error(`Failed to get daily history for ${serviceUrl}:`, err)
  }

  // Ensure we have 90 days of data (fill gaps with unknown)
  const result: DayStatus[] = []
  const now = new Date()

  for (let i = 89; i >= 0; i--) {
    const date = new Date(now.getTime() - i * 24 * 60 * 60 * 1000)
    const dateStr = date.toISOString().split('T')[0]

    const existing = history.find(h => h.date === dateStr)
    if (existing) {
      result.push(existing)
    } else {
      result.push({
        date: dateStr,
        status: 'unknown',
        uptimePercent: 0,
      })
    }
  }

  return result
}

/**
 * Get complete uptime data for a service
 */
export async function getServiceUptime(serviceName: string, serviceUrl: string): Promise<UptimeData> {
  const prometheusUrl = getPrometheusUrl()

  if (!prometheusUrl) {
    // Return mock data when Prometheus is not configured
    return {
      service: serviceName,
      uptime24h: null,
      uptime7d: null,
      uptime30d: null,
      uptime90d: null,
      dailyHistory: generateMockHistory(),
    }
  }

  // Fetch all uptime metrics in parallel
  const [uptime24h, uptime7d, uptime30d, uptime90d, dailyHistory] = await Promise.all([
    getUptimePercent(serviceUrl, '24h'),
    getUptimePercent(serviceUrl, '7d'),
    getUptimePercent(serviceUrl, '30d'),
    getUptimePercent(serviceUrl, '90d'),
    getDailyHistory(serviceUrl),
  ])

  return {
    service: serviceName,
    uptime24h,
    uptime7d,
    uptime30d,
    uptime90d,
    dailyHistory,
  }
}

/**
 * Get uptime data for multiple services
 */
export async function getAllServicesUptime(
  services: Array<{ name: string; url: string }>
): Promise<Record<string, UptimeData>> {
  const results: Record<string, UptimeData> = {}

  // Fetch all service uptime data in parallel
  const uptimePromises = services.map(async service => {
    const uptime = await getServiceUptime(service.name, service.url)
    return { name: service.name, uptime }
  })

  const uptimeResults = await Promise.all(uptimePromises)

  for (const { name, uptime } of uptimeResults) {
    results[name] = uptime
  }

  return results
}

/**
 * Generate mock history for development/fallback
 */
function generateMockHistory(): DayStatus[] {
  const history: DayStatus[] = []
  const now = new Date()

  for (let i = 89; i >= 0; i--) {
    const date = new Date(now.getTime() - i * 24 * 60 * 60 * 1000)
    const dateStr = date.toISOString().split('T')[0]

    // Generate realistic-looking uptime (mostly operational)
    const rand = Math.random()
    let status: ServiceStatus = 'operational'
    let uptimePercent = 99.5 + Math.random() * 0.5 // 99.5-100%

    if (rand < 0.02) {
      // 2% chance of degraded day
      status = 'degraded'
      uptimePercent = 95 + Math.random() * 4 // 95-99%
    } else if (rand < 0.005) {
      // 0.5% chance of outage day
      status = 'outage'
      uptimePercent = 80 + Math.random() * 15 // 80-95%
    }

    history.push({
      date: dateStr,
      status,
      uptimePercent,
    })
  }

  return history
}

/**
 * Check if Prometheus is available
 */
export async function isPrometheusAvailable(): Promise<boolean> {
  const url = getPrometheusUrl()
  if (!url) return false

  try {
    const response = await fetch(`${url}/-/ready`, {
      signal: AbortSignal.timeout(5000),
    })
    return response.ok
  } catch {
    return false
  }
}
