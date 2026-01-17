import { NextResponse } from 'next/server'
import type { NextRequest } from 'next/server'

/**
 * Dispatch Middleware - Superuser Access Control
 *
 * SECURITY: This middleware enforces strict access control.
 * Only admin@madfam.io is allowed to access Dispatch.
 *
 * The Control Tower is for infrastructure operators only.
 */

// Allowed superuser email (hardcoded for security)
const ALLOWED_SUPERUSER = 'admin@madfam.io'

// Public paths that don't require authentication
const publicPaths = [
  '/login',
  '/auth/callback',
  '/api/auth',
  '/_next',
  '/favicon.ico',
  '/public',
]

function isPublicPath(pathname: string): boolean {
  return publicPaths.some(
    (path) => pathname === path || pathname.startsWith(path + '/')
  )
}

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl

  // Skip middleware for public paths
  if (isPublicPath(pathname)) {
    return addSecurityHeaders(NextResponse.next())
  }

  // Check for authentication token
  const token = request.cookies.get('dispatch_auth')?.value
  const userEmail = request.cookies.get('dispatch_user_email')?.value

  // If no token, redirect to login
  if (!token) {
    if (pathname.startsWith('/api/')) {
      return new NextResponse(JSON.stringify({ error: 'Unauthorized' }), {
        status: 401,
        headers: { 'Content-Type': 'application/json' },
      })
    }
    return NextResponse.redirect(new URL('/login', request.url))
  }

  // SUPERUSER CHECK: Only admin@madfam.io is allowed
  if (userEmail !== ALLOWED_SUPERUSER) {
    console.warn(`[DISPATCH SECURITY] Unauthorized access attempt by: ${userEmail}`)

    if (pathname.startsWith('/api/')) {
      return new NextResponse(
        JSON.stringify({
          error: 'Forbidden',
          message: 'Dispatch access is restricted to infrastructure operators.',
        }),
        {
          status: 403,
          headers: { 'Content-Type': 'application/json' },
        }
      )
    }

    // Redirect non-superusers to an access denied page
    return NextResponse.redirect(new URL('/access-denied', request.url))
  }

  return addSecurityHeaders(NextResponse.next())
}

function addSecurityHeaders(response: NextResponse): NextResponse {
  const securityHeaders = {
    // Prevent clickjacking attacks
    'X-Frame-Options': 'DENY',

    // Prevent MIME type sniffing
    'X-Content-Type-Options': 'nosniff',

    // Enable XSS protection
    'X-XSS-Protection': '1; mode=block',

    // Control referrer information
    'Referrer-Policy': 'strict-origin-when-cross-origin',

    // Strict Content Security Policy for Dispatch
    'Content-Security-Policy': [
      "default-src 'self'",
      "script-src 'self' 'unsafe-eval' 'unsafe-inline'",
      "style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
      "font-src 'self' data: https://fonts.gstatic.com",
      "img-src 'self' data: https:",
      `connect-src 'self' http://localhost:4200 https://api.enclii.dev https://api.cloudflare.com ${process.env.NEXT_PUBLIC_JANUA_URL || 'https://api.janua.dev'}`,
      "frame-ancestors 'none'",
    ].join('; '),

    // Restrict browser features
    'Permissions-Policy': [
      'geolocation=()',
      'microphone=()',
      'camera=()',
      'payment=()',
      'usb=()',
    ].join(', '),

    // HSTS in production
    ...(process.env.NODE_ENV === 'production' && {
      'Strict-Transport-Security': 'max-age=31536000; includeSubDomains; preload',
    }),
  }

  Object.entries(securityHeaders).forEach(([key, value]) => {
    if (value) {
      response.headers.set(key, value)
    }
  })

  return response
}

export const config = {
  matcher: ['/((?!_next/static|_next/image|favicon.ico|public).*)'],
}
