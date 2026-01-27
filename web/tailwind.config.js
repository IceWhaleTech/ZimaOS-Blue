/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      fontFamily: {
        sans: ['system-ui', '-apple-system', 'BlinkMacSystemFont', 'Segoe UI', 'Roboto', 'Helvetica Neue', 'Arial', 'Noto Sans', 'sans-serif', 'Apple Color Emoji', 'Segoe UI Emoji', 'Segoe UI Symbol', 'Noto Color Emoji'],
      },
      colors: {
        // Primary palette - Dark Tech Theme
        primary: {
          DEFAULT: '#1E293B',
          light: '#334155',
          dark: '#0F172A',
        },
        // Accent colors
        accent: {
          DEFAULT: '#3B82F6',
          hover: '#2563EB',
          light: '#60A5FA',
        },
        cta: {
          DEFAULT: '#22C55E',
          hover: '#16A34A',
        },
        // Background colors
        surface: {
          base: '#0F172A',
          elevated: '#1E293B',
          card: '#334155',
        },
        // Glass effect colors
        glass: {
          bg: 'rgba(255, 255, 255, 0.05)',
          'bg-hover': 'rgba(255, 255, 255, 0.1)',
          border: 'rgba(255, 255, 255, 0.1)',
        },
      },
      borderRadius: {
        'sm': '6px',
        'md': '8px',
        'lg': '12px',
        'xl': '16px',
      },
      boxShadow: {
        'glass': '0 4px 30px rgba(0, 0, 0, 0.1)',
        'glow': '0 0 20px rgba(59, 130, 246, 0.3)',
        'glow-cta': '0 0 20px rgba(34, 197, 94, 0.3)',
      },
      backdropBlur: {
        'glass': '16px',
        'glass-lg': '20px',
      },
      transitionDuration: {
        'fast': '150ms',
        'normal': '200ms',
        'slow': '300ms',
      },
    },
  },
  plugins: [],
}
