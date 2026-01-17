'use client'

import { useAuth } from '@/contexts/AuthContext'
import { Button } from '@/components/ui/button'
import { Radio, Shield, ArrowRight } from 'lucide-react'
import { useRouter } from 'next/navigation'
import { useEffect } from 'react'

/**
 * Dispatch Login Page
 *
 * Redirects to Janua SSO for authentication.
 * Only admin@madfam.io is allowed to access Dispatch.
 */
export default function LoginPage() {
  const { isAuthenticated, isLoading, login } = useAuth()
  const router = useRouter()

  // Redirect to dashboard if already authenticated
  useEffect(() => {
    if (!isLoading && isAuthenticated) {
      router.push('/')
    }
  }, [isLoading, isAuthenticated, router])

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="size-8 border-2 border-primary border-t-transparent rounded-full animate-spin" />
      </div>
    )
  }

  return (
    <div className="min-h-screen flex items-center justify-center p-4">
      <div className="max-w-md w-full space-y-8">
        {/* Logo */}
        <div className="text-center">
          <div className="inline-flex p-4 rounded-2xl bg-primary/10 border border-primary/20 mb-6">
            <Radio className="size-12 text-primary glow-effect" />
          </div>
          <h1 className="text-3xl font-mono font-bold text-foreground tracking-tight">
            DISPATCH
          </h1>
          <p className="text-muted-foreground mt-2">
            Infrastructure Control Tower
          </p>
        </div>

        {/* Login Card */}
        <div className="rounded-xl border border-border bg-card p-6 space-y-6 card-organic">
          <div className="flex items-center gap-3 p-3 rounded-lg bg-status-warning-muted border border-status-warning/20">
            <Shield className="size-5 text-status-warning shrink-0" />
            <div className="text-sm">
              <p className="font-medium text-foreground">Restricted Access</p>
              <p className="text-muted-foreground">
                Only authorized infrastructure operators may access Dispatch.
              </p>
            </div>
          </div>

          <div className="space-y-4">
            <Button onClick={login} className="w-full gap-2" size="lg">
              Sign in with Janua SSO
              <ArrowRight className="size-4" />
            </Button>

            <p className="text-xs text-center text-muted-foreground">
              You will be redirected to{' '}
              <span className="font-mono text-primary">auth.madfam.io</span>
            </p>
          </div>
        </div>

        {/* Footer */}
        <p className="text-xs text-center text-muted-foreground">
          admin.enclii.dev | Superuser Access Only
        </p>
      </div>
    </div>
  )
}
