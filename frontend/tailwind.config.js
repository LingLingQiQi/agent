/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        'bg-primary': '#cad9f0',
        'chat-bg': '#f8f9fa',
        'sidebar-bg': '#ffffff',
        'blue-primary': '#6366f1',
        'blue-selected': '#f0f4ff',
        'text-primary': '#1a1a1a',
        'text-secondary': '#666666',
        'border-light': '#e5e7eb'
      }
    },
  },
  plugins: [
    require('@tailwindcss/typography'),
  ],
}