'use client'

import { createContext, useContext, useEffect, useState, useCallback, ReactNode } from 'react'
import { useRouter } from 'next/navigation'

/**
 * AuthContext for Dispatch
 *
 * Handles Janua SSO authentication with infrastructure operator validation.
 * Access is restricted based on:
 * 1. Email domain - must be from an allowed domain (@madfam.io by default)
 * 2. User role - must have an operator-level role (superadmin, admin, operator)
 */

const JANUA_URL = process.env.NEXT_PUBLIC_JANUA_URL || 'https://api.janua.dev'

// Allowed email domains (must match middleware configuration)
const DEFAULT_DOMAINS = ['@madfam.io']
const ALLOWED_DOMAINS = process.env.NEXT_PUBLIC_ALLOWED_ADMIN_DOMAINS
  ? process.env.NEXT_PUBLIC_ALLOWED_ADMIN_DOMAINS.split(',').map((d) => d.trim())
  : DEFAULT_DOMAINS

// Allowed roles (must match middleware configuration)
const DEFAULT_ROLES = ['superadmin', 'admin', 'operator']
const ALLOWED_ROLES = process.env.NEXT_PUBLIC_ALLOWED_ADMIN_ROLES
  ? process.env.NEXT_PUBLIC_ALLOWED_ADMIN_ROLES.split(',').map((r) => r.trim())
  : DEFAULT_ROLES

/**
 * Check if an email is from an allowed domain
 */
function isAllowedDomain(email: string): boolean {
  return ALLOWED_DOMAINS.some((domain) => email.toLowerCase().endsWith(domain.toLowerCase()))
}

/**
 * Check if user has an allowed role
 */
function hasAllowedRole(roles: string[] | undefined): boolean {
  if (!roles || roles.length === 0) return false
  return roles.some((role) => ALLOWED_ROLES.includes(role))
}

interface User {
  id: string
  email: string
  name?: string
  is_admin: boolean
  roles?: string[]
}

interface AuthContextType {
  user: User | null
  isLoading: boolean
  isAuthenticated: boolean
  isAuthorized: boolean
  login: () => void
  logout: () => void
  error: string | null
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const router = useRouter()

  // User is authorized if they have both allowed domain AND allowed role
  const isAuthorized =
    user !== null && isAllowedDomain(user.email) && hasAllowedRole(user.roles)

  // Check authentication status on mount
  useEffect(() => {
    checkAuth()
  }, [])

  const checkAuth = useCallback(async () => {
    try {
      const token = localStorage.getItem('dispatch_token')
      if (!token) {
        setIsLoading(false)
        return
      }

      // Verify token with Janua
      const response = await fetch(`${JANUA_URL}/api/v1/auth/me`, {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      })

      if (response.ok) {
        const userData = await response.json()

        // Extract roles from user data (Janua returns roles as array)
        const userRoles: string[] = userData.roles || []
        // Also check is_admin flag for backwards compatibility
        if (userData.is_admin && !userRoles.includes('admin')) {
          userRoles.push('admin')
        }

        // SECURITY: Validate domain and role
        const domainOk = isAllowedDomain(userData.email)
        const roleOk = hasAllowedRole(userRoles)

        if (!domainOk || !roleOk) {
          const reason = !domainOk
            ? 'Your email domain is not authorized for Dispatch access.'
            : 'You do not have the required role for Dispatch access.'
          setError(`Access denied. ${reason}`)
          localStorage.removeItem('dispatch_token')
          document.cookie = 'dispatch_auth=; Max-Age=0; path=/'
          document.cookie = 'dispatch_user_email=; Max-Age=0; path=/'
          document.cookie = 'dispatch_user_roles=; Max-Age=0; path=/'
          setUser(null)
        } else {
          // Set user with roles
          setUser({ ...userData, roles: userRoles })
          // Set cookies for middleware (roles as comma-separated string)
          document.cookie = `dispatch_auth=${token}; path=/; max-age=86400; SameSite=Strict`
          document.cookie = `dispatch_user_email=${userData.email}; path=/; max-age=86400; SameSite=Strict`
          document.cookie = `dispatch_user_roles=${userRoles.join(',')}; path=/; max-age=86400; SameSite=Strict`
        }
      } else {
        localStorage.removeItem('dispatch_token')
        document.cookie = 'dispatch_auth=; Max-Age=0; path=/'
        document.cookie = 'dispatch_user_email=; Max-Age=0; path=/'
        document.cookie = 'dispatch_user_roles=; Max-Age=0; path=/'
      }
    } catch (err) {
      console.error('Auth check failed:', err)
      setError('Authentication failed')
    } finally {
      setIsLoading(false)
    }
  }, [])

  const login = useCallback(() => {
    // Redirect to Janua login with Dispatch as the redirect target
    const redirectUri = encodeURIComponent(`${window.location.origin}/auth/callback`)
    window.location.href = `${JANUA_URL}/oauth/authorize?redirect_uri=${redirectUri}&client_id=dispatch`
  }, [])

  const logout = useCallback(async () => {
    try {
      const token = localStorage.getItem('dispatch_token')
      if (token) {
        // Notify Janua of logout
        await fetch(`${JANUA_URL}/api/v1/auth/logout`, {
          method: 'POST',
          headers: {
            Authorization: `Bearer ${token}`,
          },
        }).catch(() => {})
      }
    } finally {
      localStorage.removeItem('dispatch_token')
      document.cookie = 'dispatch_auth=; Max-Age=0; path=/'
      document.cookie = 'dispatch_user_email=; Max-Age=0; path=/'
      document.cookie = 'dispatch_user_roles=; Max-Age=0; path=/'
      setUser(null)
      router.push('/login')
    }
  }, [router])

  return (
    <AuthContext.Provider
      value={{
        user,
        isLoading,
        isAuthenticated: !!user,
        isAuthorized,
        login,
        logout,
        error,
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}
