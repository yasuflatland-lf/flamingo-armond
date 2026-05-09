# Routing topology

> Part of [`docs/frontend.md`](../frontend.md). See the index for related chapters.

`/` (HomePage RSC) is the canonical landing — every entry converges there, and `/` then routes the user to the most useful next screen rather than dropping them on a static page. The convergence guarantees no stale-placeholder or redirect-loop edge case can develop:

| Entry | Anonymous → | Signed-in → |
|---|---|---|
| `/` (`app/page.tsx`) | `/login` | 4-branch redirect (see below) |
| `/login` (`app/login/page.tsx`) | render `LoginButton` | `/` (delegating the post-login routing decision back to HomePage) |
| `/auth/callback?code=...` (`app/auth/callback/route.ts`) | n/a | `next` query value, defaulting to `/cardgroups` |
| `/cards/new?cardgroup=<id>` | `/login` | render chip + `CardForm`; resolves cardgroup via 4-priority chain (see `/cards/new` below) |
| `/cardgroups/new?welcome=1` | `/login` | render new-cardgroup form with welcome copy (onboarding entry) |
| Header "Admin" link (admin only) | hidden | `/admin` (then `/admin/layout.tsx` gate → `/admin/page.tsx` → `redirect("/admin/users")`) |

### HomePage 4-branch redirect

`/` (HomePage RSC) fetches `MeWithLastViewedQuery` and chooses the next screen in this order:

1. `me.lastViewedCardgroup != null` → `redirect("/learn/${id}")` — returning users land directly on the swipe UI, skipping the cardgroup list.
2. `myCardgroups.length > 0` → `redirect("/cardgroups")` — user owns cardgroups but has never used `/learn`; they need to pick one.
3. else → `redirect("/cardgroups/new?welcome=1")` — onboarding for first-time users.
4. (matrix corner) anonymous → `redirect("/login")`, handled before the GraphQL fetch.

Loop prevention: HomePage only redirects *outward* — never to `/` — so the chain terminates in one hop. `/learn/[id]` and `/cardgroups` do not redirect back to `/`, so a returning user's two-hop login flow is `/login → / → /learn/[id]`.

### `/cards/new` cardgroup resolution (4-priority chain)

The global "+ Card" CTA lands on `/cards/new` (the FAB does **not** carry `?cardgroup=...` — the route owns the resolution). The page resolves the cardgroup id in this order:

1. `searchParams.cardgroup` — accepted only if the id appears in the user's `myCardgroups` list. A non-owned id silently falls through (no `BAD_USER_INPUT` surface) so a stale URL after sharing or revoke does not 500.
2. `me.lastViewedCardgroup.id` — same ownership check (defensive; the FK already cascades).
3. User has cardgroups but neither (1) nor (2) resolved → render the chip in undetermined state and **force the picker open** so the user explicitly chooses.
4. `myCardgroups.length === 0` → `redirect("/cardgroups/new?welcome=1")` — the same onboarding target as HomePage branch (3).

The URL `?cardgroup=<id>` is the **single source of truth** for the chip + form pair: the picker calls `router.replace("/cards/new?cardgroup=<newId>", { scroll: false })`, and chip / form re-render against the new URL. No client-side state holds a duplicate "selected cardgroup" — eliminates the chip-vs-form drift class of bugs.

`/login` redirects signed-in users back to `/` (not directly to `/cardgroups`) so the post-login branching lives in exactly one place — HomePage. A second branching site in `/login` would have to repeat the `lastViewedCardgroup` lookup and would silently rot when HomePage's logic evolves.

The legacy "render `/` with health check inline" pattern is replaced by `/api/healthz` — see "Route Handler conventions" below. External monitors that polled `/` must move to `/api/healthz`.

The admin entry surfaces (desktop `AdminPill`, mobile hamburger admin section) are the only UI affordances for entering `/admin`. Both render only when `gqlFetch(HeaderMeQuery)` returns a role named `"admin"`. The `/admin/layout.tsx` server-side gate is the enforcement boundary — header visibility is a UI hint, not security. See `.claude/rules/frontend-rsc-error-handling.md` for the failure-mode contract that lets the Header degrade silently when the role lookup fails.

### Global navigation primitives

Three components live under `frontend/src/components/nav/` and compose into the root layout:

- `GlobalHeader` — RSC; rendered from `app/layout.tsx`. Mobile shows hamburger + logo + truncated email; desktop shows logo + nav links + `+ Card` CTA + `AdminPill` (when admin) + `LogoutButton`. Header MUST degrade silently on `getUser()` or `me`-query failure — see the rule.
- `GlobalFAB` — Client; floats bottom-right with the coral `--brand-primary` background. Hidden on `/login`, `/learn/*`, `/admin/*`, `/cards/new`, `/cardgroups/new` — i.e. routes that are anonymous-only, full-bleed UI, a different audience, or the FAB's own destination (would loop). Hide list lives in one regex (`HIDDEN_PATH_RE`) inside `global-fab.tsx`; there is no allow-list. The FAB also wraps its button in an `md:hidden` container so the desktop header `+ Card` CTA is the **single** add-card affordance at `>= md` — see "Breakpoint-exclusive primary action" below. The FAB does NOT pre-pend `?cardgroup=...` — `/cards/new` owns the 4-priority resolution; passing the id from the FAB would create two truths.
- `HeaderAddCardLink` — Client; the desktop `+ Card` CTA. Lives in `frontend/src/components/nav/header-add-card-link.tsx` so the surrounding `GlobalHeader` can stay an RSC. Reads `usePathname()` and applies the "current location CTA" pattern below when on `/cards/new`.
- `HamburgerDrawer` — Client; mobile-only. Three visually-divided groups separated by `<hr>`: (1) primary nav (Cardgroups, Profile), (2) admin entry tinted with `bg-brand-tint` + `border-brand-tint-border` when `isAdmin`, (3) sign-out. The admin tint is a non-CTA use of the brand palette — see "Design tokens" below.

`AdminPill` is a server component rendered inline in the desktop header when `isAdmin === true`. Its tint comes from the same `--brand-tint*` family as the hamburger admin section so the two surfaces are visually linked.

#### Breakpoint-exclusive primary action

When two surfaces would render the same primary CTA at the same time (here: the desktop header `+ Card` button and the mobile `GlobalFAB`), make them **mutually exclusive by Tailwind class** (`md:hidden` on the FAB, `hidden md:flex` would go on a desktop-only nav element) rather than by a JS `useMediaQuery` hook. Two reasons: (1) SSR and CSR render the same markup, so there is no hydration flash where both affordances briefly appear; (2) no React re-render fires on viewport change — the browser handles the transition in CSS. Reach for `useMediaQuery` only when the *content* (not just visibility) differs across breakpoints, which is rare.

#### Current-location CTA — `aria-current="page"` plus disabled styling

When a primary CTA's destination is the page the user is already on, do not hide it (the layout would jump and the user loses orientation) and do not disable it as a `<button>` (it is a `<Link>`, not a button). Apply `aria-current="page"` for screen-reader semantics and pair with `pointer-events-none opacity-60` to make the same `<Link>` visually subdued and unclickable. Drop the hover variant on the same branch — a hover-color flash on a non-interactive element is dishonest. `HeaderAddCardLink` is the reference implementation.

#### Two-layer hide pattern: static-path regex + dynamic-segment helper

The FAB's "do not show this control" rule has two distinct flavours: (a) static path patterns where the decision depends only on the literal pathname (`/login`, `/learn/*`, `/admin/*`, `/cards/new`, `/cardgroups/new`, `/profile`), and (b) dynamic-segment patterns where the decision depends on parsing the segment (`/cardgroups/<id>/edit` — the FAB has nothing meaningful to add on an edit form). Encoding both in one regex makes the regex grow monotonically with every new edit-shaped path; encoding both in `resolveFabAction` mixes "what should the FAB do here" with "should the FAB exist here". The cleaner split is:

- `HIDDEN_PATH_RE` in `frontend/src/components/nav/global-fab.tsx` — static-path allowlist of *paths to hide* (regex over the literal pathname).
- `resolveFabAction(pathname)` in `frontend/src/components/nav/fab-action.ts` — returns `null` for dynamic patterns where no FAB action makes sense (today: the cardgroup-edit shape).

`GlobalFAB` short-circuits on `HIDDEN_PATH_RE.test(pathname)` first, then on `resolveFabAction(pathname) === null`.

**Cross-layer interaction must be tested explicitly.** A path that is suppressed by ONE layer but produces a non-null/non-suppressed value at the OTHER layer is the failure mode this split introduces. Today's example: `/cardgroups/new` is hidden by `HIDDEN_PATH_RE` but `resolveFabAction("/cardgroups/new")` returns `{ kind: "card-with-group", cardgroupId: "new" }` — the literal `"new"` is treated as a cardgroup id by `CARDGROUP_DETAIL_RE`. Without an explicit cross-layer test, a future contributor "consolidating" both layers into one might inadvertently expose the FAB on the new-cardgroup form. The test in `frontend/src/components/nav/fab-action.test.ts` (`is shadowed externally by GlobalFAB's hidden-path guard for /cardgroups/new`) documents that responsibility-split — it is the only place the split is explicit; production code looks consistent either way.

### Cardgroup chip + picker primitives

`CardgroupChip` (client) and `CardgroupPickerSheet` (client) live under `frontend/src/components/cardgroups/`. The chip is a short pill with the current cardgroup name + `ChevronDown`; tapping it opens the picker. The picker is a shadcn `Sheet` rendered with `side="bottom"` on all viewport sizes (the original plan considered a desktop `Dialog` variant via `useMediaQuery`, but a bottom sheet works on both and avoids dragging in a media-query hook just for one component). It is backed by `MyCardgroupsQuery` via `useQuery`. Two consumers exist today: `/cards/new` (chip + form pair, single source of truth in URL) and any future page that needs cardgroup-scoped writes from a non-cardgroup-scoped route.

The chip truncates names to `max-w-[12ch] sm:max-w-[20ch]`; long names show as ellipsis-on-mobile and the full name appears in the picker. There is no tooltip — tapping the chip already reveals the full list.

### Cardgroup creation flow — two entry points

Cardgroup creation entry points are intentionally asymmetric because card and cardgroup creation frequencies are roughly 95:5. Two surfaces initiate cardgroup creation:

1. **From `/cardgroups` list** (page-shell `primaryActions` slot): The "+ New cardgroup" link is rendered in the `<ListingPageShell>` header `primaryActions` cluster — visible in BOTH the populated and empty-state branches of the list. Clicking it navigates to `/cardgroups/new` without a `returnTo` query, and the completion page is `/cardgroups/{newId}` — the new cardgroup's detail page. The earlier dashed empty-state CTA and the populated-state footer link were both consolidated into this single shell-level affordance, so the empty state and the populated state surface exactly one create entry point at the same screen position.
2. **From `/cards/new` picker** (always-visible inline link): The `CardgroupPickerSheet` always displays a "+ Create new cardgroup…" link at the bottom, even when cardgroups exist. Clicking it navigates to `/cardgroups/new?returnTo=/cards/new`, embedding the return destination into the query parameter. After creation, the page redirects to `/cards/new?cardgroup={newId}` with the new cardgroup pre-selected in the form, keeping the user in the card creation flow without a detour.

**Open-redirect guard**: The `returnTo` query is sanitized server-side in `frontend/src/app/cardgroups/new/page.tsx` via the `sanitizeReturnTo` function. Only paths starting with `/` (and not `//` or `/\`) are accepted; everything else is rejected and defaults to `/cardgroups/{newId}`. The `/\` rejection blocks the browser-normalised backslash bypass — Chrome and Firefox rewrite `/\evil.com` to `//evil.com` and follow the protocol-relative URL off-domain.

The asymmetry reflects the information hierarchy: the header "+ Card" button and mobile FAB remain the global, frequent path; cardgroup creation is a setup-level operation accessible only when creating a card (picker) or managing the cardgroup list (/cardgroups). This surfaces the card → cardgroup parent-child relationship in the interaction flow.

### Design tokens — brand palette usage rules

`globals.css` defines five brand tokens (`--brand-primary`, `--brand-primary-foreground`, `--brand-tint`, `--brand-tint-border`, `--brand-tint-foreground`). The product palette is intentionally narrow:

- `--brand-primary` (coral `#FE7F70` via OKLCH) — primary CTAs only: `+ Card` FAB, the `+ Card` desktop nav button, and primary form Save buttons. Not for body text, links, or hover states.
- `--brand-tint*` — low-volume brand-identity surfaces: (1) the sign-in brand panel (`/login` left column), (2) admin entry surfaces (`AdminPill`, the drawer admin section). The tint signals "this surface carries product identity or marks a context switch" without competing with a primary CTA. Use `bg-brand-tint` for the panel background, `text-brand-tint-foreground` for headings, and `text-brand-tint-foreground/80` for secondary copy on that background.

Every other surface uses shadcn's slate-based defaults (`--primary`, `--secondary`, `--accent`). New components should only reach for the brand tokens when they fall into one of the categories above — adding a fourth use site for `--brand-tint*` or a second category for `--brand-primary` dilutes the signal, so reviewers should push back unless the new surface is clearly identity-bearing or a primary CTA.

### Welcome copy on `/cardgroups/new?welcome=1`

The `?welcome=1` query parameter makes `/cardgroups/new` (already the cardgroup-create page) double as the onboarding screen for first-time users by conditionally rendering a welcome banner above the form. HomePage branch (3) and `/cards/new` priority (4) both target this URL. Without the query parameter, the page renders only the form — same behaviour as before. Driving the difference from the URL keeps onboarding statelessly bookmarkable / sharable and avoids a separate `/welcome` route whose only difference would be the copy.

### Sign-in page layout (`/login`)

`/login` bypasses `AppShell` (root layout short-circuits when `pathname === "/login"`) and owns the entire viewport. The page uses a split-screen shell:

```tsx
<main data-testid="login-grid" className="relative grid h-svh lg:grid-cols-2">
  <div className="max-lg:hidden ... bg-brand-tint">{/* brand panel */}</div>
  <div className="flex flex-col">{/* form column: h1, error banner, OAuth button, footer */}</div>
</main>
```

Three rules apply to any future page that adopts this shell (e.g. `/signup`, `/reset-password`):

1. **`h-svh`, not `h-screen` or `min-h-screen`.** `h-screen` resolves to `100vh`, which on Safari mobile includes the address-bar height that collapses on scroll — a non-scrolling full-viewport page measured against `100vh` ends up taller than the visible area and the bottom content is cut off. `h-svh` (small viewport height) is the always-visible height and is the correct unit for a fixed-viewport login shell. Use `min-h-svh` only when the content can grow taller than the viewport.
2. **Brand panel hidden below `lg` with `max-lg:hidden`.** The left column hides below `lg`; the right column is always visible. This keeps the mobile experience a single-column form (no wasted vertical space for branding) while letting desktop carry the full-bleed brand panel.
3. **`<main>` landmark required.** Because `AppShell` is bypassed, the page is the only place a `<main>` landmark can be emitted — see [`docs/frontend/rsc-error-handling/pages-bypassing-appshell-must-render-main.md`](./rsc-error-handling/pages-bypassing-appshell-must-render-main.md).

