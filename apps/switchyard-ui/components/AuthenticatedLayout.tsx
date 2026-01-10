'use client';

import { useEffect } from 'react';
import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { useAuth } from '@/contexts/AuthContext';
import { ThemeToggle } from '@/components/theme/theme-toggle';
import { NotificationBell } from '@/components/notifications/notification-bell';
import { CommandPalette } from '@/components/command/command-palette';

interface AuthenticatedLayoutProps {
  children: React.ReactNode;
}

export function AuthenticatedLayout({ children }: AuthenticatedLayoutProps) {
  const pathname = usePathname();
  const router = useRouter();
  const { user, isAuthenticated, isLoading, logout } = useAuth();

  const navigation = [
    { name: 'Dashboard', href: '/' },
    { name: 'Projects', href: '/projects' },
    { name: 'Services', href: '/services' },
    { name: 'Deployments', href: '/deployments' },
    { name: 'Domains', href: '/domains' },
    { name: 'Observability', href: '/observability' },
    { name: 'Activity', href: '/activity' },
  ];

  const secondaryNav = [
    { name: 'Usage', href: '/usage' },
    { name: 'Settings', href: '/settings' },
  ];

  // Redirect to login if not authenticated
  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      router.push('/login');
    }
  }, [isLoading, isAuthenticated, router]);

  const handleLogout = async () => {
    await logout();
    router.push('/login');
  };

  // Show loading state
  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-enclii-blue mx-auto"></div>
          <p className="mt-4 text-muted-foreground">Loading...</p>
        </div>
      </div>
    );
  }

  // Don't render protected content if not authenticated
  if (!isAuthenticated) {
    return null;
  }

  return (
    <>
      <nav className="bg-background shadow-sm border-b border-border">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex justify-between h-16">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <Link href="/" className="flex items-center">
                  <span className="text-2xl font-bold text-enclii-blue">🚂 Enclii</span>
                  <span className="ml-2 text-sm text-muted-foreground font-medium">Switchyard</span>
                </Link>
              </div>
              <div className="ml-10 flex items-baseline space-x-4">
                {navigation.map((item) => {
                  const isActive = pathname === item.href || (item.href !== '/' && pathname.startsWith(item.href));
                  return (
                    <Link
                      key={item.name}
                      href={item.href}
                      className={`px-3 py-2 text-sm font-medium transition-colors duration-150 ${
                        isActive
                          ? 'text-enclii-blue border-b-2 border-enclii-blue'
                          : 'text-muted-foreground hover:text-enclii-blue hover:border-b-2 hover:border-border'
                      }`}
                    >
                      {item.name}
                    </Link>
                  );
                })}
              </div>
            </div>
            <div className="flex items-center space-x-4">
              {/* Command Palette */}
              <CommandPalette />

              {/* Secondary Navigation */}
              <div className="flex items-center space-x-2 mr-4 border-r border-border pr-4">
                {secondaryNav.map((item) => {
                  const isActive = pathname === item.href || pathname.startsWith(item.href);
                  return (
                    <Link
                      key={item.name}
                      href={item.href}
                      className={`px-2 py-1 text-sm font-medium transition-colors duration-150 rounded ${
                        isActive
                          ? 'text-enclii-blue bg-accent'
                          : 'text-muted-foreground hover:text-enclii-blue hover:bg-accent'
                      }`}
                    >
                      {item.name}
                    </Link>
                  );
                })}
              </div>

              <div className="flex items-center text-sm text-muted-foreground">
                <div className="w-2 h-2 bg-green-500 rounded-full mr-2"></div>
                <span>System Healthy</span>
              </div>

              {/* Notifications */}
              <NotificationBell />

              {/* Theme Toggle */}
              <ThemeToggle />

              {/* User Menu */}
              <div className="relative flex items-center gap-3">
                <span className="text-sm text-muted-foreground">
                  {user?.name || user?.email}
                </span>
                <button
                  onClick={handleLogout}
                  className="text-sm text-muted-foreground hover:text-foreground px-3 py-1 rounded border border-border hover:bg-accent transition-colors"
                >
                  Sign out
                </button>
              </div>
            </div>
          </div>
        </div>
      </nav>
      <main className="min-h-screen">{children}</main>
      <footer className="bg-background border-t border-border mt-12">
        <div className="max-w-7xl mx-auto py-6 px-4 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between">
            <div className="text-sm text-muted-foreground">
              © {new Date().getFullYear()} Enclii Platform. Built with ❤️ for developers.
            </div>
            <div className="flex items-center space-x-4 text-sm text-muted-foreground">
              <a href="https://docs.enclii.dev" className="hover:text-foreground">Documentation</a>
              <a href="https://api.enclii.dev/docs" className="hover:text-foreground">API</a>
              <a href="https://status.enclii.dev" className="hover:text-foreground">Status</a>
            </div>
          </div>
        </div>
      </footer>
    </>
  );
}
