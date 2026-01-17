'use client'

import { useAuth } from '@/contexts/AuthContext'
import { DomainMatrix } from '@/components/domain-matrix'
import { Button } from '@/components/ui/button'
import {
  Radio,
  Globe,
  Server,
  Shield,
  Activity,
  LogOut,
  User,
} from 'lucide-react'
import { useRouter } from 'next/navigation'
import { useEffect } from 'react'

/**
 * Dispatch - The Control Tower
 *
 * Infrastructure management dashboard for superusers only.
 * Provides sovereign control over domains, tunnels, and ecosystem resources.
 */
export default function DispatchDashboard() {
  const { user, isLoading, isAuthenticated, isSuperuser, logout, error } = useAuth()
  const router = useRouter()

  // Redirect to login if not authenticated
  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      router.push('/login')
    }
  }, [isLoading, isAuthenticated, router])

  // Show loading state
  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="flex flex-col items-center gap-4">
          <div className="size-8 border-2 border-primary border-t-transparent rounded-full animate-spin" />
          <p className="text-muted-foreground font-mono text-sm">
            Initializing Dispatch<span className="terminal-cursor" />
          </p>
        </div>
      </div>
    )
  }

  // Show auth error
  if (error) {
    return (
      <div className="min-h-screen flex items-center justify-center p-4">
        <div className="max-w-md w-full rounded-lg border border-destructive/30 bg-destructive/10 p-6 text-center">
          <Shield className="size-12 mx-auto mb-4 text-destructive" />
          <h1 className="text-xl font-semibold text-foreground mb-2">Access Denied</h1>
          <p className="text-muted-foreground mb-4">{error}</p>
          <Button variant="outline" onClick={() => router.push('/login')}>
            Return to Login
          </Button>
        </div>
      </div>
    )
  }

  // Main dashboard
  return (
    <div className="min-h-screen bg-background">
      {/* Header */}
      <header className="border-b border-border bg-card/50 backdrop-blur-sm sticky top-0 z-50">
        <div className="container mx-auto px-4 py-3 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-primary/10 border border-primary/20">
              <Radio className="size-5 text-primary glow-effect" />
            </div>
            <div>
              <h1 className="font-mono font-semibold text-foreground text-lg tracking-tight">
                DISPATCH
              </h1>
              <p className="text-xs text-muted-foreground">Control Tower</p>
            </div>
          </div>

          <div className="flex items-center gap-4">
            {/* User Info */}
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <User className="size-4" />
              <span className="font-mono">{user?.email}</span>
              {isSuperuser && (
                <span className="px-1.5 py-0.5 rounded text-xs bg-primary/20 text-primary border border-primary/30">
                  SUPERUSER
                </span>
              )}
            </div>
            <Button variant="ghost" size="sm" onClick={logout} className="gap-2">
              <LogOut className="size-4" />
              Logout
            </Button>
          </div>
        </div>
      </header>

      {/* Navigation */}
      <nav className="border-b border-border bg-card/30">
        <div className="container mx-auto px-4">
          <div className="flex items-center gap-1">
            <NavTab icon={Globe} label="Domains" active />
            <NavTab icon={Server} label="Tunnels" />
            <NavTab icon={Activity} label="Health" />
            <NavTab icon={Shield} label="Security" />
          </div>
        </div>
      </nav>

      {/* Main Content */}
      <main className="container mx-auto px-4 py-8">
        <DomainMatrix />
      </main>

      {/* Footer */}
      <footer className="border-t border-border py-4 mt-auto">
        <div className="container mx-auto px-4">
          <div className="flex items-center justify-between text-xs text-muted-foreground">
            <span className="font-mono">Dispatch v0.1.0 | admin.enclii.dev</span>
            <span>
              Powered by{' '}
              <span className="text-primary">Cloudflare</span> +{' '}
              <span className="text-primary">Enclii</span>
            </span>
          </div>
        </div>
      </footer>
    </div>
  )
}

// Navigation Tab Component
function NavTab({
  icon: Icon,
  label,
  active = false,
}: {
  icon: React.ComponentType<{ className?: string }>
  label: string
  active?: boolean
}) {
  return (
    <button
      className={`flex items-center gap-2 px-4 py-3 text-sm font-medium border-b-2 transition-colors ${
        active
          ? 'border-primary text-primary'
          : 'border-transparent text-muted-foreground hover:text-foreground hover:border-border'
      }`}
    >
      <Icon className="size-4" />
      {label}
    </button>
  )
}
