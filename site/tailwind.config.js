/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,jsx}"],
  theme: {
    extend: {
      fontFamily: {
        title: [
          "-apple-system",
          "BlinkMacSystemFont",
          '"SF Pro Display"',
          '"Helvetica Neue"',
          "Helvetica",
          "Arial",
          "ui-sans-serif",
          "system-ui",
          "sans-serif",
        ],
        geist: [
          "-apple-system",
          "BlinkMacSystemFont",
          '"SF Pro Text"',
          '"Helvetica Neue"',
          "Helvetica",
          "Arial",
          "ui-sans-serif",
          "system-ui",
          "sans-serif",
        ],
      },
      colors: {
        flamingo: {
          50: "#fff7f4",
          100: "#ffe8df",
          200: "#ffd0c2",
          300: "#ffab96",
          400: "#ff8068",
          500: "#ff5f4f",
          600: "#f04438",
          700: "#d92d20",
        },
        ink: "#242424",
        mute: "#848484",
      },
      boxShadow: {
        soft: "0px 3px 12.9px 0px #97979714",
        button: "0px 2px 10.1px 0px #ff5f4f33",
        float: "0px 24px 70px rgba(36,36,36,0.10)",
      },
      keyframes: {
        marquee: {
          "0%": { transform: "translateX(0)" },
          "100%": { transform: "translateX(-50%)" },
        },
        reveal: {
          "0%": { opacity: "0", filter: "blur(8px)", transform: "translateY(10px)" },
          "100%": { opacity: "1", filter: "blur(0)", transform: "translateY(0)" },
        },
        float: {
          "0%, 100%": { transform: "translateY(0)" },
          "50%": { transform: "translateY(-8px)" },
        },
        swipe: {
          "0%, 100%": { transform: "rotate(-4deg) translateX(0)" },
          "50%": { transform: "rotate(4deg) translateX(12px)" },
        },
      },
      animation: {
        marquee: "marquee 55s linear infinite",
        reveal: "reveal .75s ease forwards",
        float: "float 5s ease-in-out infinite",
        swipe: "swipe 4.5s ease-in-out infinite",
      },
    },
  },
};
