import type { NextConfig } from 'next'

const nextConfig: NextConfig = {
  reactStrictMode: true,
  output: 'standalone',

  // Environment variables for Dispatch
  env: {
    // Dispatch runs on port 4203 (admin.enclii.dev)
    NEXT_PUBLIC_APP_URL: process.env.NEXT_PUBLIC_APP_URL || 'http://localhost:4203',
    // Janua SSO for authentication
    NEXT_PUBLIC_JANUA_URL: process.env.NEXT_PUBLIC_JANUA_URL || 'https://api.janua.dev',
    // Cloudflare API (server-side only via env)
    CLOUDFLARE_API_TOKEN: process.env.CLOUDFLARE_API_TOKEN || '',
    CLOUDFLARE_ACCOUNT_ID: process.env.CLOUDFLARE_ACCOUNT_ID || '',
  },
}

export default nextConfig
