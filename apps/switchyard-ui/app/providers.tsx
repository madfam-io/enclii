'use client';

import { ThemeProvider } from 'next-themes';
import { AuthProvider } from '@/contexts/AuthContext';
import { ScopeProvider } from '@/contexts/ScopeContext';

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <ThemeProvider
      attribute="class"
      defaultTheme="dark"
      enableSystem
      disableTransitionOnChange
    >
      <AuthProvider>
        <ScopeProvider>{children}</ScopeProvider>
      </AuthProvider>
    </ThemeProvider>
  );
}
