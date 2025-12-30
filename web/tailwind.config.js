/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        'nofx-gold': '#F0B90B',
        'nofx-gold-dim': 'rgba(240, 185, 11, 0.15)',
        'nofx-bg': '#0B0E11',
        'nofx-accent': '#00F0FF',
        'nofx-text': '#EAECEF',
      },
    },
  },
  plugins: [],
}
