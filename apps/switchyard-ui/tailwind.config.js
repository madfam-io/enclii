/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    './pages/**/*.{js,ts,jsx,tsx,mdx}',
    './components/**/*.{js,ts,jsx,tsx,mdx}',
    './app/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  theme: {
    extend: {
      colors: {
        'enclii-blue': '#0070f3',
        'enclii-green': '#00b894',
        'enclii-orange': '#e17055',
        'enclii-red': '#d63031',
      },
    },
  },
  plugins: [],
}