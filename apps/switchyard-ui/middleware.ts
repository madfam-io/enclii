import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

/**
 * Next.js Middleware for security headers and authentication
 *
 * SECURITY: This middleware runs on every request to add security headers
 * and can be extended to handle authentication checks
 */
export function middleware(request: NextRequest) {
  const response = NextResponse.next();

  // SECURITY FIX: Add comprehensive security headers
  const securityHeaders = {
    // Prevent clickjacking attacks
    "X-Frame-Options": "DENY",

    // Prevent MIME type sniffing
    "X-Content-Type-Options": "nosniff",

    // Enable XSS protection (legacy but still useful)
    "X-XSS-Protection": "1; mode=block",

    // Control referrer information
    "Referrer-Policy": "strict-origin-when-cross-origin",

    // Content Security Policy - restricts resource loading
    "Content-Security-Policy": [
      "default-src 'self'",
      "script-src 'self' 'unsafe-eval' 'unsafe-inline'", // Next.js requires unsafe-eval in dev
      "style-src 'self' 'unsafe-inline'", // Tailwind requires unsafe-inline
      "img-src 'self' data: https:",
      "font-src 'self' data:",
      "connect-src 'self' http://localhost:8001", // API endpoint (port 8001 per PORT_REGISTRY)
      "frame-ancestors 'none'",
    ].join("; "),

    // Permissions Policy - restrict browser features
    "Permissions-Policy": [
      "geolocation=()",
      "microphone=()",
      "camera=()",
      "payment=()",
      "usb=()",
      "magnetometer=()",
      "gyroscope=()",
      "accelerometer=()",
    ].join(", "),

    // HSTS - Force HTTPS (only in production)
    ...(process.env.NODE_ENV === "production" && {
      "Strict-Transport-Security":
        "max-age=31536000; includeSubDomains; preload",
    }),
  };

  // Apply all security headers
  Object.entries(securityHeaders).forEach(([key, value]) => {
    response.headers.set(key, value);
  });

  // TODO: Add authentication check here
  // Example:
  // const token = request.cookies.get('auth_token');
  // if (!token && !isPublicPath(request.nextUrl.pathname)) {
  //   return NextResponse.redirect(new URL('/login', request.url));
  // }

  return response;
}

// Configure which routes the middleware should run on
export const config = {
  matcher: [
    /*
     * Match all request paths except:
     * - _next/static (static files)
     * - _next/image (image optimization files)
     * - favicon.ico (favicon file)
     * - public folder
     */
    "/((?!_next/static|_next/image|favicon.ico|public).*)",
  ],
};
