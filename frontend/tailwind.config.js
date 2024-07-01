/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      keyframes: {
        flip: {
          '0%': { transform: 'rotateY(0deg)' },
          '12.5%': { transform: 'rotateY(45deg)' },
          '25%': { transform: 'rotateY(90deg)' },
          '37.5%': { transform: 'rotateY(135deg)' },
          '50%': { transform: 'rotateY(180deg)' },
          '62.5%': { transform: 'rotateY(225deg)' },
          '75%': { transform: 'rotateY(270deg)' },
          '87.5%': { transform: 'rotateY(315deg)' },
          '100%': { transform: 'rotateY(360deg)' },
        },
      },
      animation: {
        flip: 'flip 2s infinite linear',
      },
    },
  },
  plugins: [],
}