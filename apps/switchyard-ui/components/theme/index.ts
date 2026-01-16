/**
 * Theme System - Dual-Skin Architecture
 *
 * Exports:
 * - ThemeProvider: Wraps app with dual-skin + next-themes support
 * - ThemeToggle: Light/Dark mode toggle (legacy)
 * - CombinedThemeToggle: Combined skin + light/dark toggle
 * - useThemeSkin: Hook for accessing current theme skin
 */

export { ThemeProvider, ThemeSkinToggle, useThemeSkin } from './theme-provider';
export { ThemeToggle, CombinedThemeToggle } from './theme-toggle';
