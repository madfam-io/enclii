'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { useAuth } from '@/contexts/AuthContext';
import { ThemeToggle } from '@/components/theme/theme-toggle';
import { NotificationBell } from '@/components/notifications/notification-bell';
import { CommandPalette } from '@/components/command/command-palette';
import { SystemHealthBadge } from '@/components/dashboard/system-health';
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Menu, ChevronDown } from 'lucide-react';

interface AuthenticatedLayoutProps {
  children: React.ReactNode;
}

interface NavItem {
  name: string;
  href: string;
}

// Helper component for nav links
function NavLink({ item, pathname }: { item: NavItem; pathname: string }) {
  const isActive = pathname === item.href || (item.href !== '/' && pathname.startsWith(item.href));
  return (
    <Link
      href={item.href}
      className={`px-3 py-2 text-sm font-medium transition-colors duration-150 whitespace-nowrap ${
        isActive
          ? 'text-enclii-blue border-b-2 border-enclii-blue'
          : 'text-muted-foreground hover:text-enclii-blue hover:border-b-2 hover:border-border'
      }`}
    >
      {item.name}
    </Link>
  );
}

export function AuthenticatedLayout({ children }: AuthenticatedLayoutProps) {
  const pathname = usePathname();
  const router = useRouter();
  const { user, isAuthenticated, isLoading, logout } = useAuth();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  // Primary navigation - always visible at lg+ breakpoint
  const primaryNav: NavItem[] = [
    { name: 'Dashboard', href: '/' },
    { name: 'Projects', href: '/projects' },
    { name: 'Services', href: '/services' },
    { name: 'Deployments', href: '/deployments' },
    { name: 'Observability', href: '/observability' },
  ];

  // Overflow navigation - in dropdown at lg, visible at xl+
  const overflowNav: NavItem[] = [
    { name: 'Templates', href: '/templates' },
    { name: 'Databases', href: '/databases' },
    { name: 'Functions', href: '/functions' },
    { name: 'Domains', href: '/domains' },
    { name: 'Activity', href: '/activity' },
  ];

  // Combined navigation for mobile menu
  const navigation = [...primaryNav, ...overflowNav];

  const secondaryNav: NavItem[] = [
    { name: 'Usage', href: '/usage' },
    { name: 'Settings', href: '/settings' },
  ];

  // Check if any overflow item is active (for "More" button highlighting)
  const isOverflowActive = overflowNav.some(
    (item) => pathname === item.href || pathname.startsWith(item.href)
  );

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
    <div className="min-h-screen flex flex-col">
      <nav className="bg-background shadow-sm border-b border-border">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex justify-between h-16">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <Link href="/" className="flex items-center">
                  <span className="text-2xl font-bold text-enclii-blue">🚂 Enclii</span>
                  <span className="ml-2 text-sm text-muted-foreground font-medium hidden sm:inline">Switchyard</span>
                </Link>
              </div>
              {/* Desktop Navigation - Hidden on mobile/tablet */}
              <div className="hidden lg:flex ml-6 items-baseline space-x-1 xl:space-x-4">
                {/* Primary nav items - always visible at lg+ */}
                {primaryNav.map((item) => (
                  <NavLink key={item.name} item={item} pathname={pathname} />
                ))}

                {/* More dropdown - visible at lg, hidden at xl */}
                <div className="xl:hidden">
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <button
                        className={`px-3 py-2 text-sm font-medium transition-colors duration-150 flex items-center gap-1 ${
                          isOverflowActive
                            ? 'text-enclii-blue'
                            : 'text-muted-foreground hover:text-enclii-blue'
                        }`}
                      >
                        More
                        <ChevronDown className="h-4 w-4" />
                      </button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="start" className="w-40">
                      {overflowNav.map((item) => {
                        const isActive = pathname === item.href || pathname.startsWith(item.href);
                        return (
                          <DropdownMenuItem key={item.name} asChild>
                            <Link
                              href={item.href}
                              className={isActive ? 'text-enclii-blue bg-accent' : ''}
                            >
                              {item.name}
                            </Link>
                          </DropdownMenuItem>
                        );
                      })}
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>

                {/* Overflow items - visible at xl+ */}
                <div className="hidden xl:flex items-baseline space-x-4">
                  {overflowNav.map((item) => (
                    <NavLink key={item.name} item={item} pathname={pathname} />
                  ))}
                </div>
              </div>
            </div>
            <div className="flex items-center space-x-1 lg:space-x-2">
              {/* Command Palette - Always visible */}
              <CommandPalette />

              {/* Secondary Navigation - Hidden on mobile */}
              <div className="hidden md:flex items-center space-x-1 mr-2 border-r border-border pr-2 lg:mr-4 lg:pr-4">
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

              {/* System Health - Hidden on mobile/tablet, visible at lg+ */}
              <div className="hidden lg:block">
                <SystemHealthBadge />
              </div>

              {/* Notifications - Always visible */}
              <NotificationBell />

              {/* Theme Toggle - Always visible */}
              <ThemeToggle />

              {/* User Menu - Hidden until xl, shown in hamburger otherwise */}
              <div className="hidden xl:flex relative items-center gap-2">
                <span className="text-sm text-muted-foreground truncate max-w-[120px]">
                  {user?.name || user?.email}
                </span>
                <button
                  onClick={handleLogout}
                  className="text-sm text-muted-foreground hover:text-foreground px-2 py-1 rounded border border-border hover:bg-accent transition-colors"
                >
                  Sign out
                </button>
              </div>

              {/* Mobile/Tablet Hamburger Menu - visible below xl */}
              <Sheet open={mobileMenuOpen} onOpenChange={setMobileMenuOpen}>
                <SheetTrigger asChild>
                  <button className="xl:hidden p-2 rounded-md hover:bg-accent">
                    <Menu className="h-6 w-6 text-foreground" />
                    <span className="sr-only">Open menu</span>
                  </button>
                </SheetTrigger>
                <SheetContent side="right" className="w-[300px] sm:w-[350px]">
                  <SheetHeader>
                    <SheetTitle>Menu</SheetTitle>
                  </SheetHeader>
                  <nav className="flex flex-col gap-4 mt-6">
                    {/* Navigation Links */}
                    <div className="space-y-1">
                      <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-2">
                        Navigation
                      </p>
                      {navigation.map((item) => {
                        const isActive = pathname === item.href || (item.href !== '/' && pathname.startsWith(item.href));
                        return (
                          <Link
                            key={item.name}
                            href={item.href}
                            onClick={() => setMobileMenuOpen(false)}
                            className={`block px-3 py-2 rounded-md text-sm font-medium transition-colors ${
                              isActive
                                ? 'bg-accent text-enclii-blue'
                                : 'text-foreground hover:bg-accent'
                            }`}
                          >
                            {item.name}
                          </Link>
                        );
                      })}
                    </div>

                    {/* Secondary Navigation */}
                    <div className="space-y-1 border-t border-border pt-4">
                      <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-2">
                        Settings
                      </p>
                      {secondaryNav.map((item) => {
                        const isActive = pathname === item.href || pathname.startsWith(item.href);
                        return (
                          <Link
                            key={item.name}
                            href={item.href}
                            onClick={() => setMobileMenuOpen(false)}
                            className={`block px-3 py-2 rounded-md text-sm font-medium transition-colors ${
                              isActive
                                ? 'bg-accent text-enclii-blue'
                                : 'text-foreground hover:bg-accent'
                            }`}
                          >
                            {item.name}
                          </Link>
                        );
                      })}
                    </div>

                    {/* System Status - visible in mobile menu */}
                    <div className="border-t border-border pt-4">
                      <div className="px-3 py-2 flex items-center gap-2">
                        <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                          System Status
                        </span>
                        <SystemHealthBadge />
                      </div>
                    </div>

                    {/* User Section */}
                    <div className="border-t border-border pt-4 mt-auto">
                      <div className="px-3 py-2">
                        <p className="text-sm font-medium text-foreground truncate">
                          {user?.name || 'User'}
                        </p>
                        <p className="text-xs text-muted-foreground truncate">
                          {user?.email}
                        </p>
                      </div>
                      <button
                        onClick={() => {
                          setMobileMenuOpen(false);
                          handleLogout();
                        }}
                        className="w-full mt-2 px-3 py-2 text-sm font-medium text-destructive hover:bg-destructive/10 rounded-md transition-colors text-left"
                      >
                        Sign out
                      </button>
                    </div>
                  </nav>
                </SheetContent>
              </Sheet>
            </div>
          </div>
        </div>
      </nav>
      <main className="flex-grow">{children}</main>
      <footer className="bg-background border-t border-border">
        <div className="max-w-7xl mx-auto py-6 px-4 sm:px-6 lg:px-8">
          <div className="flex flex-col sm:flex-row items-center justify-between gap-4">
            <div className="text-sm text-muted-foreground text-center sm:text-left">
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
    </div>
  );
}
