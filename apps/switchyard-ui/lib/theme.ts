/**
 * Theme System - Dual-Skin Architecture
 *
 * Supports two distinct visual modes:
 * - "enterprise" (Default OSS): Vercel-like neutrality
 * - "solarpunk" (MADFAM Production): Bioluminescent organic industrial
 *
 * Controlled via:
 * 1. Environment variable: NEXT_PUBLIC_THEME_DEFAULT
 * 2. User preference in localStorage
 * 3. data-theme attribute on <body>
 */

export type ThemeSkin = 'enterprise' | 'solarpunk';
export type ThemeMode = 'light' | 'dark' | 'system';

const THEME_STORAGE_KEY = 'enclii-theme-skin';
const DEFAULT_THEME = (process.env.NEXT_PUBLIC_THEME_DEFAULT as ThemeSkin) || 'enterprise';

/**
 * Get the current theme skin from storage or environment default
 */
export function getThemeSkin(): ThemeSkin {
  if (typeof window === 'undefined') return DEFAULT_THEME;

  const stored = localStorage.getItem(THEME_STORAGE_KEY) as ThemeSkin | null;
  if (stored && (stored === 'enterprise' || stored === 'solarpunk')) {
    return stored;
  }

  return DEFAULT_THEME;
}

/**
 * Set the theme skin and apply to document
 */
export function setThemeSkin(skin: ThemeSkin): void {
  if (typeof window === 'undefined') return;

  localStorage.setItem(THEME_STORAGE_KEY, skin);
  document.body.setAttribute('data-theme', skin);

  // Dispatch custom event for components to react
  window.dispatchEvent(new CustomEvent('theme-skin-change', { detail: { skin } }));
}

/**
 * Initialize theme on page load
 * Call this in your root layout or _app
 */
export function initializeTheme(): ThemeSkin {
  const skin = getThemeSkin();
  if (typeof window !== 'undefined') {
    document.body.setAttribute('data-theme', skin);
  }
  return skin;
}

/**
 * Check if currently using Solarpunk theme
 */
export function isSolarpunk(): boolean {
  if (typeof window === 'undefined') return DEFAULT_THEME === 'solarpunk';
  return document.body.getAttribute('data-theme') === 'solarpunk';
}

/**
 * Theme-aware class helper
 * Returns different classes based on current theme
 */
export function themeClass(enterprise: string, solarpunk: string): string {
  return isSolarpunk() ? solarpunk : enterprise;
}

/**
 * Get theme-specific icon/emoji
 * Useful for "Nutrients" vs "Resources" labeling
 */
export const THEME_LABELS = {
  enterprise: {
    cpu: 'CPU',
    memory: 'Memory',
    storage: 'Storage',
    bandwidth: 'Bandwidth',
    buildMinutes: 'Build Minutes',
  },
  solarpunk: {
    cpu: '🌞 Sunlight',
    memory: '💧 Water',
    storage: '🌍 Soil',
    bandwidth: '🌬️ Air Flow',
    buildMinutes: '⚡ Energy',
  },
} as const;

export function getResourceLabel(
  resource: keyof typeof THEME_LABELS.enterprise,
  useOrganic: boolean = false
): string {
  if (useOrganic && isSolarpunk()) {
    return THEME_LABELS.solarpunk[resource];
  }
  return THEME_LABELS.enterprise[resource];
}
