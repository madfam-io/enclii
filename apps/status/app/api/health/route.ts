import { NextResponse } from 'next/server'

/**
 * Health check endpoint for the status page itself
 * Used by Kubernetes probes
 */
export async function GET() {
  return NextResponse.json({
    status: 'healthy',
    timestamp: new Date().toISOString(),
    service: 'enclii-status',
    version: process.env.npm_package_version || '0.1.0',
  })
}
