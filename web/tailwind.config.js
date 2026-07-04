/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        sage: {
          bg: '#e6e9e3',
          shadow: '#c3c6bd',
          highlight: '#ffffff',
          text: '#3c3f37',
          accent: '#4a604f', /* Muted sage-green for active text */
        }
      },
      boxShadow: {
        'neu-raised': '8px 8px 16px #c3c6bd, -8px -8px 16px #ffffff',
        'neu-pressed': 'inset 6px 6px 12px #c3c6bd, inset -6px -6px 12px #ffffff',
        'neu-raised-sm': '4px 4px 8px #c3c6bd, -4px -4px 8px #ffffff',
        'neu-pressed-sm': 'inset 3px 3px 6px #c3c6bd, inset -3px -3px 6px #ffffff',
      },
      fontFamily: {
        fraunces: ['Fraunces', 'serif'],
        manrope: ['Manrope', 'sans-serif'],
        mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'Monaco', 'Consolas', 'monospace'],
      }
    },
  },
  plugins: [],
}