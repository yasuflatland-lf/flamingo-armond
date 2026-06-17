export default {
  plugins: {
    // Tailwind CSS v4 ships its PostCSS integration as a separate package; v4
    // also handles @import inlining and vendor prefixing internally, so the
    // standalone postcss-import / autoprefixer plugins are no longer needed.
    "@tailwindcss/postcss": {},
  },
};
