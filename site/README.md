# Landing page (`site/`)

Static marketing landing page for 🦩 flamingo-armond, published to GitHub Pages.

## How it is published

`.github/workflows/landing.yml` deploys this folder to the **root** of the
`gh-pages` branch on every push to `main` that touches `site/**`. The ER chart
(`er-chart.yml`) publishes to `gh-pages/er-chart`, so both share one Pages site:

| Path | Served at | Source |
|---|---|---|
| `/` | <https://yasuflatland-lf.github.io/flamingo-armond/> | `site/` (this folder) |
| `/er-chart/` | <https://yasuflatland-lf.github.io/flamingo-armond/er-chart/> | `er-chart.yml` |

The landing workflow's `clean-exclude` preserves `er-chart/` (and the legacy
`vercel.json`) when it refreshes the root, so the two never clobber each other.

## Layout

```
site/
  index.html          React + Tailwind (CDN) single-page LP; assets referenced by path
  .nojekyll           disables Jekyll processing for the whole Pages site
  videos/             H.264 MP4, isom brand, +faststart (moov at front for fast autoplay)
    learn-demo.mp4
    cardgroup-demo.mp4
  images/             carousel screenshots
    desktop-learn.png  desktop-login.png  mobile-manage.png  mobile-learn.png
```

## Asset rules

- **Never re-embed media as `data:` base64.** The original export inlined both
  demo videos as base64, ballooning the HTML to ~8.4 MB and wrecking LCP.
  Videos and screenshots are external files referenced by relative path; the
  landing workflow fails the build if a base64 `video/mp4` blob reappears in
  `index.html`.
- **Videos are remuxed, not re-encoded.** Source recordings are QuickTime-brand
  H.264. They are remuxed losslessly to ISO MP4 (`isom`) with `+faststart`:
  ```bash
  ffmpeg -i source.mov -c copy -movflags +faststart -f mp4 site/videos/<name>.mp4
  ```
  `+faststart` moves the `moov` atom to the front so the browser can start
  playback before the whole file downloads. The `isom` brand (vs the original
  `qt`) is required for Firefox to decode the file when served as `video/mp4`.
- The inline favicon and decorative SVGs stay inline — they are a few KB each
  and inlining avoids extra round-trips.

## Local preview

```bash
cd site && python3 -m http.server 8000
# open http://localhost:8000
```

## Optional: Cloudflare CDN + custom domain

GitHub Pages already serves this site over its global CDN. To front it with a
custom domain on Cloudflare (extra DDoS mitigation, edge rules, cache control):

1. Add a `CNAME` file to this folder containing the apex/subdomain (e.g.
   `www.example.com`) and set the same domain under repo **Settings → Pages →
   Custom domain**.
2. In Cloudflare DNS, point the domain at GitHub Pages:
   - apex: four `A` records — `185.199.108.153`, `185.199.109.153`,
     `185.199.110.153`, `185.199.111.153`
   - or `www` as `CNAME` → `yasuflatland-lf.github.io`
3. Set Cloudflare **SSL/TLS mode to Full** (not Flexible — Flexible causes a
   redirect loop with the GitHub Pages HTTPS enforcement). `Full (Strict)` works
   once GitHub has provisioned the Pages certificate for the domain.
