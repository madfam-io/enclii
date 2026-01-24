import { NextRequest, NextResponse } from 'next/server'
import type { Incident, IncidentStatus, IncidentSeverity } from '@/lib/types'

// In-memory store for development
// In Phase 3, this will be replaced with database operations
const incidents: Incident[] = []

/**
 * Get all incidents
 *
 * Query params:
 * - status: Filter by status (investigating, identified, monitoring, resolved)
 * - limit: Maximum number of incidents to return (default: 50)
 * - offset: Number of incidents to skip (default: 0)
 */
export async function GET(request: NextRequest) {
  const { searchParams } = new URL(request.url)
  const status = searchParams.get('status') as IncidentStatus | null
  const limit = parseInt(searchParams.get('limit') || '50', 10)
  const offset = parseInt(searchParams.get('offset') || '0', 10)

  let filtered = [...incidents]

  // Filter by status if provided
  if (status) {
    filtered = filtered.filter((i) => i.status === status)
  }

  // Sort by created date (newest first)
  filtered.sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime())

  // Apply pagination
  const paginated = filtered.slice(offset, offset + limit)

  return NextResponse.json({
    incidents: paginated,
    total: filtered.length,
    limit,
    offset,
  })
}

/**
 * Create a new incident
 *
 * Body:
 * - title: Incident title (required)
 * - severity: minor | major | critical (required)
 * - affectedServices: Array of service names (required)
 * - message: Initial update message (optional)
 *
 * Note: This endpoint will require admin authentication in Phase 3
 */
export async function POST(request: NextRequest) {
  try {
    const body = await request.json()

    // Validate required fields
    if (!body.title) {
      return NextResponse.json(
        { error: 'title is required' },
        { status: 400 }
      )
    }

    if (!body.severity || !['minor', 'major', 'critical'].includes(body.severity)) {
      return NextResponse.json(
        { error: 'severity must be one of: minor, major, critical' },
        { status: 400 }
      )
    }

    if (!body.affectedServices || !Array.isArray(body.affectedServices)) {
      return NextResponse.json(
        { error: 'affectedServices must be an array' },
        { status: 400 }
      )
    }

    // Create incident
    const now = new Date().toISOString()
    const incident: Incident = {
      id: crypto.randomUUID(),
      title: body.title,
      status: 'investigating',
      severity: body.severity as IncidentSeverity,
      affectedServices: body.affectedServices,
      createdAt: now,
      updates: [],
    }

    // Add initial update if provided
    if (body.message) {
      incident.updates.push({
        id: crypto.randomUUID(),
        incidentId: incident.id,
        message: body.message,
        status: 'investigating',
        createdAt: now,
      })
    }

    incidents.push(incident)

    return NextResponse.json(incident, { status: 201 })
  } catch {
    return NextResponse.json(
      { error: 'Invalid request body' },
      { status: 400 }
    )
  }
}

/**
 * Update an incident
 *
 * Body:
 * - id: Incident ID (required)
 * - status: New status (optional)
 * - message: Update message (optional, creates a new update)
 *
 * Note: This endpoint will require admin authentication in Phase 3
 */
export async function PATCH(request: NextRequest) {
  try {
    const body = await request.json()

    if (!body.id) {
      return NextResponse.json(
        { error: 'id is required' },
        { status: 400 }
      )
    }

    const incident = incidents.find((i) => i.id === body.id)

    if (!incident) {
      return NextResponse.json(
        { error: 'Incident not found' },
        { status: 404 }
      )
    }

    const now = new Date().toISOString()

    // Update status if provided
    if (body.status) {
      const validStatuses: IncidentStatus[] = ['investigating', 'identified', 'monitoring', 'resolved']
      if (!validStatuses.includes(body.status)) {
        return NextResponse.json(
          { error: 'Invalid status' },
          { status: 400 }
        )
      }
      incident.status = body.status

      // Set resolved timestamp if resolving
      if (body.status === 'resolved') {
        incident.resolvedAt = now
      }
    }

    // Add update if message provided
    if (body.message) {
      incident.updates.push({
        id: crypto.randomUUID(),
        incidentId: incident.id,
        message: body.message,
        status: body.status || incident.status,
        createdAt: now,
      })
    }

    return NextResponse.json(incident)
  } catch {
    return NextResponse.json(
      { error: 'Invalid request body' },
      { status: 400 }
    )
  }
}

/**
 * Delete an incident
 *
 * Query params:
 * - id: Incident ID to delete
 *
 * Note: This endpoint will require admin authentication in Phase 3
 */
export async function DELETE(request: NextRequest) {
  const { searchParams } = new URL(request.url)
  const id = searchParams.get('id')

  if (!id) {
    return NextResponse.json(
      { error: 'id is required' },
      { status: 400 }
    )
  }

  const index = incidents.findIndex((i) => i.id === id)

  if (index === -1) {
    return NextResponse.json(
      { error: 'Incident not found' },
      { status: 404 }
    )
  }

  incidents.splice(index, 1)

  return NextResponse.json({ success: true })
}
