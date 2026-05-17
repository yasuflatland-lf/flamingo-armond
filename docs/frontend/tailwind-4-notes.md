# Tailwind 4 notes

> Part of [`frontend/CLAUDE.md`](../../frontend/CLAUDE.md). See the index for related chapters.

- Tokens and dark-mode variant live in `src/app/globals.css` via `@theme` and `@custom-variant dark (...)`. There is no `tailwind.config.ts`.
- PostCSS plugin is `@tailwindcss/postcss` (the old `tailwindcss` plugin no longer exists in v4). `autoprefixer` is not needed — Tailwind 4 handles prefixing internally.
- Tokens map into the Tailwind namespace via an **`@theme inline`** block in `globals.css` (`inline` matters — without it Tailwind emits duplicate variables). `tw-animate-css` is pulled in with `@import "tw-animate-css"` (it is a CSS package, not a JS plugin).
- When new shadcn components are added, extend the `@theme` block with the additional `--color-*` tokens the components reference.

