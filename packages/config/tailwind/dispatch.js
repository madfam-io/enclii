/**
 * Dispatch Tailwind extensions
 *
 * Extends base config with dispatch-specific features:
 * - Terminal aesthetics
 * - Mono-first fonts
 * - Terminal-specific animations
 */
const base = require('./base');

/** @type {import('tailwindcss').Config} */
module.exports = {
  ...base,
  content: [
    './pages/**/*.{js,ts,jsx,tsx,mdx}',
    './components/**/*.{js,ts,jsx,tsx,mdx}',
    './app/**/*.{js,ts,jsx,tsx,mdx}',
    // Shared UI components
    '../../packages/ui-components/src/**/*.{ts,tsx}',
  ],
  theme: {
    extend: {
      ...base.theme.extend,
      fontFamily: {
        ...base.theme.extend.fontFamily,
        // Dispatch uses mono for display (terminal aesthetic)
        mono: ['var(--font-geist-mono)', 'JetBrains Mono', 'monospace'],
        display: ['JetBrains Mono', 'var(--font-geist-mono)', 'monospace'],
      },
      animation: {
        ...base.theme.extend.animation,
        'terminal-blink': 'terminal-blink 1s step-end infinite',
      },
      keyframes: {
        ...base.theme.extend.keyframes,
        'terminal-blink': {
          '0%, 100%': { opacity: '1' },
          '50%': { opacity: '0' },
        },
      },
    },
  },
};
