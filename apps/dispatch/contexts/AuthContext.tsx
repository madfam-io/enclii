'use client'

import { createContext, useContext, useEffect, useState, useCallback, ReactNode } from 'react'
import { useRouter } from 'next/navigation'

/**
 * AuthContext for Dispatch
 *
 * Handles Janua SSO authentication with superuser validation.
 * Only admin@madfam.io is allowed to use Dispatch.
 */

const JANUA_URL = process.env.NEXT_PUBLIC_JANUA_URL || 'https://api.janua.dev'
const ALLOWED_SUPERUSER = 'admin@madfam.io'

interface User {
  id: string
  email: string
  name?: string
  is_admin: boolean
}

interface AuthContextType {
  user: User | null
  isLoading: boolean
  isAuthenticated: boolean
  isSuperuser: boolean
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

  const isSuperuser = user?.email === ALLOWED_SUPERUSER

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

        // SECURITY: Only allow admin@madfam.io
        if (userData.email !== ALLOWED_SUPERUSER) {
          setError('Access denied. Dispatch is restricted to infrastructure operators.')
          localStorage.removeItem('dispatch_token')
          document.cookie = 'dispatch_auth=; Max-Age=0; path=/'
          document.cookie = 'dispatch_user_email=; Max-Age=0; path=/'
          setUser(null)
        } else {
          setUser(userData)
          // Set cookies for middleware
          document.cookie = `dispatch_auth=${token}; path=/; max-age=86400; SameSite=Strict`
          document.cookie = `dispatch_user_email=${userData.email}; path=/; max-age=86400; SameSite=Strict`
        }
      } else {
        localStorage.removeItem('dispatch_token')
        document.cookie = 'dispatch_auth=; Max-Age=0; path=/'
        document.cookie = 'dispatch_user_email=; Max-Age=0; path=/'
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
        isSuperuser,
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
