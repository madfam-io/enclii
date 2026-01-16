'use client';

import * as React from 'react';
import { ThemeProvider as NextThemesProvider } from 'next-themes';
import { type ThemeSkin, initializeTheme, setThemeSkin, getThemeSkin } from '@/lib/theme';

// =============================================================================
// DUAL-SKIN THEME PROVIDER
//
// Wraps next-themes with our dual-skin system (Enterprise vs Solarpunk).
// Manages both:
// 1. Light/Dark mode (via next-themes)
// 2. Theme skin (Enterprise vs Solarpunk via data-theme attribute)
// =============================================================================

interface ThemeSkinContextValue {
  skin: ThemeSkin;
  setSkin: (skin: ThemeSkin) => void;
  isSolarpunk: boolean;
}

const ThemeSkinContext = React.createContext<ThemeSkinContextValue | undefined>(undefined);

export function useThemeSkin(): ThemeSkinContextValue {
  const context = React.useContext(ThemeSkinContext);
  if (!context) {
    throw new Error('useThemeSkin must be used within a ThemeProvider');
  }
  return context;
}

interface ThemeProviderProps {
  children: React.ReactNode;
  /** Default theme skin (overridden by NEXT_PUBLIC_THEME_DEFAULT env var) */
  defaultSkin?: ThemeSkin;
  /** Storage key for theme preference */
  storageKey?: string;
}

export function ThemeProvider({
  children,
  defaultSkin,
  storageKey = 'enclii-theme',
}: ThemeProviderProps) {
  const [skin, setSkinState] = React.useState<ThemeSkin>('enterprise');
  const [mounted, setMounted] = React.useState(false);

  // Initialize theme on mount
  React.useEffect(() => {
    const initialSkin = initializeTheme();
    setSkinState(initialSkin);
    setMounted(true);
  }, []);

  // Listen for theme changes from other tabs/windows
  React.useEffect(() => {
    const handleStorageChange = (e: StorageEvent) => {
      if (e.key === 'enclii-theme-skin' && e.newValue) {
        setSkinState(e.newValue as ThemeSkin);
        document.body.setAttribute('data-theme', e.newValue);
      }
    };

    const handleThemeChange = (e: CustomEvent<{ skin: ThemeSkin }>) => {
      setSkinState(e.detail.skin);
    };

    window.addEventListener('storage', handleStorageChange);
    window.addEventListener('theme-skin-change' as any, handleThemeChange);

    return () => {
      window.removeEventListener('storage', handleStorageChange);
      window.removeEventListener('theme-skin-change' as any, handleThemeChange);
    };
  }, []);

  const setSkin = React.useCallback((newSkin: ThemeSkin) => {
    setSkinState(newSkin);
    setThemeSkin(newSkin);
  }, []);

  const value = React.useMemo(
    () => ({
      skin,
      setSkin,
      isSolarpunk: skin === 'solarpunk',
    }),
    [skin, setSkin]
  );

  // Prevent flash of wrong theme
  if (!mounted) {
    return (
      <div style={{ visibility: 'hidden' }}>
        {children}
      </div>
    );
  }

  return (
    <ThemeSkinContext.Provider value={value}>
      <NextThemesProvider
        attribute="class"
        defaultTheme="system"
        enableSystem
        storageKey={storageKey}
        disableTransitionOnChange={false}
      >
        {children}
      </NextThemesProvider>
    </ThemeSkinContext.Provider>
  );
}

// =============================================================================
// THEME SKIN TOGGLE - Switch between Enterprise and Solarpunk
// =============================================================================

interface ThemeSkinToggleProps {
  className?: string;
}

export function ThemeSkinToggle({ className }: ThemeSkinToggleProps) {
  const { skin, setSkin, isSolarpunk } = useThemeSkin();

  return (
    <button
      onClick={() => setSkin(isSolarpunk ? 'enterprise' : 'solarpunk')}
      className={className}
      title={`Switch to ${isSolarpunk ? 'Enterprise' : 'Solarpunk'} theme`}
      aria-label={`Current theme: ${skin}. Click to switch.`}
    >
      {isSolarpunk ? (
        <span className="flex items-center gap-2">
          <span className="text-solarpunk-chlorophyll">🌿</span>
          <span className="text-xs">Solarpunk</span>
        </span>
      ) : (
        <span className="flex items-center gap-2">
          <span>🏢</span>
          <span className="text-xs">Enterprise</span>
        </span>
      )}
    </button>
  );
}
