/**
 * Switchyard UI Tailwind extensions
 *
 * Extends base config with switchyard-specific features:
 * - Brand colors
 * - Trellis visualization colors
 * - Additional animations
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
      colors: {
        ...base.theme.extend.colors,
        // Brand colors
        'enclii-blue': '#0070f3',
        'enclii-green': '#00b894',
        'enclii-orange': '#e17055',
        'enclii-red': '#d63031',

        // Trellis visualization colors
        trellis: {
          node: 'hsl(var(--trellis-node))',
          'node-hover': 'hsl(var(--trellis-node-hover))',
          connector: 'hsl(var(--trellis-connector))',
          'connector-active': 'hsl(var(--trellis-connector-active))',
        },
      },
      animation: {
        ...base.theme.extend.animation,
        'trellis-grow': 'trellis-grow 0.5s ease-out forwards',
        'node-appear': 'node-appear 0.3s ease-out forwards',
      },
      keyframes: {
        ...base.theme.extend.keyframes,
        'trellis-grow': {
          '0%': { strokeDashoffset: '100%', opacity: '0' },
          '100%': { strokeDashoffset: '0%', opacity: '1' },
        },
        'node-appear': {
          '0%': { transform: 'scale(0.8)', opacity: '0' },
          '100%': { transform: 'scale(1)', opacity: '1' },
        },
      },
    },
  },
};
