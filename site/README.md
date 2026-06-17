# Landing page (`site/`)

Static marketing landing page for 🦩 flamingo-armond, built with **Vite + React**
and published to GitHub Pages. It is intentionally separate from the Next.js app
in `frontend/`: it shares no domain, auth, or data layer, only the brand palette,
and it keeps its own dependency tree, lockfile, and deploy cadence.

## Why a build step (not a single CDN-loaded HTML file)

This page was previously a single `index.html` that loaded React, ReactDOM,
`@babel/standalone`, and the Tailwind Play CDN at **runtime**, transpiling JSX in
the browser. That shipped React's development build to production and made the
live page hostage to external CDN version drift — an unpinned `@babel/standalone`
following a major bump to Babel 8 once rendered the page blank. Vite moves all
compilation to **build time** against pinned dependencies, so the published
output is a self-contained set of minified, content-hashed assets with no runtime
CDN dependency.

## How it is published

`.github/workflows/landing.yml` runs `pnpm install --frozen-lockfile` + `pnpm build`
in `site/`, then deploys the build output (`site/dist/`) to the **root** of the
`gh-pages` branch on every push to `main` that touches `site/**`. The ER chart
(`er-chart.yml`) publishes to `gh-pages/er-chart`, so both share one Pages site:

| Path | Served at | Source |
|---|---|---|
| `/` | <https://yasuflatland-lf.github.io/flamingo-armond/> | `site/dist/` (built from this folder) |
| `/er-chart/` | <https://yasuflatland-lf.github.io/flamingo-armond/er-chart/> | `er-chart.yml` |

The landing workflow's `clean-exclude` preserves `er-chart/` when it refreshes the
root, so the two never clobber each other. `vite.config.js` sets `base: './'` so
the hashed asset URLs resolve under the `/flamingo-armond/` project sub-path.

## Layout

```
site/
  index.html           Vite entry: <head> meta + inline favicon, <div id="root">, module script
  vite.config.js       base: './' + @vitejs/plugin-react
  tailwind.config.js   brand palette / shadows / keyframes (Tailwind v3)
  postcss.config.js    tailwindcss + autoprefixer
  package.json         pinned deps; "packageManager" pins pnpm for corepack
  pnpm-workspace.yaml  marks site/ as its own workspace root (isolated from frontend)
  src/
    main.jsx           createRoot mount
    App.jsx            the page (ported verbatim from the old inline JSX)
    index.css          @tailwind directives + the page's custom CSS
  public/              copied verbatim into dist/ (kept out of the bundle)
    .nojekyll          disables Jekyll for the whole Pages site
    vercel.json        ignoreCommand "exit 0" (tells Vercel to skip the gh-pages branch)
    videos/            learn-demo.mp4  cardgroup-demo.mp4
    images/            desktop-learn.png  desktop-login.png  mobile-manage.png  mobile-learn.png
  dist/                build output (git-ignored; the published artifact)
```

## Asset rules

- **Never re-embed media as `data:` base64.** The original export inlined both
  demo videos as base64, ballooning the HTML to ~8.4 MB and wrecking LCP. Videos
  and screenshots live in `public/` and are referenced by relative path; the
  landing workflow fails the build if a base64 `video/mp4` blob reappears in
  `site/dist`.
- **Videos are remuxed, not re-encoded.** Source recordings are QuickTime-brand
  H.264. They are remuxed losslessly to ISO MP4 (`isom`) with `+faststart`:
  ```bash
  ffmpeg -i source.mov -c copy -movflags +faststart -f mp4 site/public/videos/<name>.mp4
  ```
  `+faststart` moves the `moov` atom to the front so the browser can start
  playback before the whole file downloads. The `isom` brand (vs the original
  `qt`) is required for Firefox to decode the file when served as `video/mp4`.
- The inline favicon and decorative SVGs stay inline — they are a few KB each and
  inlining avoids extra round-trips.

## Local development

```bash
cd site
pnpm install        # corepack provisions the pinned pnpm; esbuild build is allow-listed
pnpm dev            # Vite dev server with HMR
pnpm build          # emit dist/
pnpm preview        # serve dist/ locally to verify the production build
```
