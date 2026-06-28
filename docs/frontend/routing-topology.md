# Routing topology

> Part of [`frontend/CLAUDE.md`](../../frontend/CLAUDE.md). See the index for related chapters.

`/` (HomePage RSC) is the canonical landing — every entry converges there, and `/` then routes the user to the most useful next screen rather than dropping them on a static page. The convergence guarantees no stale-placeholder or redirect-loop edge case can develop:

| Entry | Anonymous → | Signed-in → |
|---|---|---|
| `/` (`app/page.tsx`) | `/login` | redirect chain (see below) |
| `/login` (`app/login/page.tsx`) | render `LoginButton` | `/` (delegating the post-login routing decision back to HomePage) |
| `/auth/callback?code=...` (`app/auth/callback/route.ts`) | n/a | `next` query value, defaulting to `/` (so HomePage owns the post-OAuth landing decision) |
| `/onboarding` (`app/onboarding/page.tsx`) | `/login` | render `OnboardingForm`; already-onboarded users redirect to `/` (self-guard via `isUserOnboarded`) |
| `/onboarding/start` (`app/onboarding/start/page.tsx`) | `/login` | first-deck chooser: import a master-catalog deck (→ `/learn/{id}`) or start with the default decks (seeds the `is_default_starter` decks → `/cardgroups`, or → `/cardgroups/new?welcome=1` when zero were seeded); not-onboarded self-guard → `/onboarding`; empty catalog → `/cardgroups/new?welcome=1` |
| `/cards/new?cardgroup=<id>` | `/login` | render chip + `CardForm`; resolves cardgroup via 4-priority chain (see `/cards/new` below) |
| `/cardgroups/new?welcome=1` | `/login` | render new-cardgroup form with welcome copy (post-onboarding entry) |
| Admin entry (rail item, admin only) | hidden | `/admin/<sub>` (no `/admin` shim — see "Unified admin layout" in [`profile-page-profile.md`](./profile-page-profile.md)) |

### HomePage redirect chain

`/` (HomePage RSC) is the single decision point for "where does this user belong next". It fetches `MeWithLastViewedQuery` and walks the chain in priority order:

```
HomePage (/)
├─ no Supabase session            → /login
├─ !isUserOnboarded(me)           → /onboarding          (highest signed-in priority)
├─ me.lastViewedCardgroup != null → /learn/{id}
├─ myCardgroupsConnection.totalCount > 0 → /cardgroups
└─ else                           → /onboarding/start
```

The terminal `else` (onboarded, no `lastViewedCardgroup`, zero cardgroups) lands on `/onboarding/start`, the first-deck chooser — see its row in the table above. The chooser lets the user import a master-catalog deck (→ `/learn/{id}`) or seed the published default-starter decks (landing on `/cardgroups`, or on `/cardgroups/new?welcome=1` when zero defaults were seeded); when the master catalog is empty the page falls through to `/cardgroups/new?welcome=1`.

Why `isUserOnboarded` is the highest signed-in priority: a user whose `displayName` was nulled by an admin (or who completed OAuth but never finished `/onboarding`) would otherwise land on `/learn/{lastViewedCardgroup}` or `/cardgroups` with an empty profile — visible to other users — and have no in-product affordance to fix it. The gate sits ahead of the cardgroup branches so the empty-`displayName` state is structurally unreachable on any signed-in surface.

The completion predicate is `isUserOnboarded(me)` in `frontend/src/lib/auth/onboarding.ts` — see [`onboarding-gate.md`](./onboarding-gate.md) for why the same predicate also self-guards `/onboarding` (preventing onboarded users who deep-link there from sitting on a no-op form).

Loop prevention: HomePage only redirects *outward* — never to `/` — so the chain terminates in one hop. `/learn/[id]`, `/cardgroups`, and `/onboarding` do not redirect back to `/`, so a returning user's two-hop login flow is `/login → / → /learn/[id]`. `/onboarding` itself redirects already-onboarded users *outward* to `/`, where the chain runs again and lands them on the right screen.

The OAuth callback at `/auth/callback` defaults its post-exchange redirect to `/` (not directly to `/cardgroups`) so the post-sign-in routing decision lives in exactly one place — HomePage. A second branching site in the callback would have to repeat both the `lastViewedCardgroup` lookup and the onboarding check, and would silently rot whenever HomePage's logic evolves.

### `/cards/new` cardgroup resolution (4-priority chain)

The nav-header "+" resolves its target from the current pathname via `resolveHeaderCreateAction` (`frontend/src/components/nav/header-create-action.ts`) — there is no single generic `card → /cards/new` catch-all. The per-route behaviour is:

- `/cardgroups` → `/cardgroups/new` (cardgroup creation; **no** `?cardgroup=...`).
- `/cardgroups/:id/edit` → `/cards/new?cardgroup=<enc>` (card creation pre-filled with the cardgroup id, single-encoded).
- `/learn/:id` → `/cards/new?cardgroup=<enc>&return=/learn/<enc>` (same, plus a `return` param so the new-card flow can redirect back to the learn screen).
- `/admin/roles` → opens the create sheet via `?new=true` (no `href` — role creation has no separate-page target).
- any other route → no "+" (`resolveHeaderCreateAction` returns `null`).

On the `/cards/new?cardgroup=<id>` targets above, the id is supplied by the originating route (edit / learn), and the page still re-runs the ownership-checked resolution below. The page resolves the cardgroup id in this order:

1. `searchParams.cardgroup` — accepted only if the id appears in the user's `myCardgroupsConnection` edges. A non-owned id silently falls through (no `BAD_USER_INPUT` surface) so a stale URL after sharing or revoke does not 500.
2. `me.lastViewedCardgroup.id` — same ownership check (defensive; the FK already cascades).
3. User has cardgroups but neither (1) nor (2) resolved → render the chip in undetermined state and **force the picker open** so the user explicitly chooses.
4. `myCardgroupsConnection.totalCount === 0` → `redirect("/cardgroups/new?welcome=1")` — the post-onboarding first-cardgroup screen. (HomePage's "no cardgroups yet" branch routes through `/onboarding/start` first, which falls through to this same screen when the catalog is empty or the user chooses to create their own.)

The URL `?cardgroup=<id>` is the **single source of truth** for the chip + form pair: the picker calls `router.replace("/cards/new?cardgroup=<newId>", { scroll: false })`, and chip / form re-render against the new URL. No client-side state holds a duplicate "selected cardgroup" — eliminates the chip-vs-form drift class of bugs.

`/login` (when hit by an already-signed-in user) follows the same single-decision-point rule: it redirects to `/`, never directly to `/cardgroups` or `/learn/...`, for the same reason the OAuth callback does.

### Top-level nav destination wiring

A new top-level authenticated destination (e.g. `/catalog`) is not discoverable until it is wired into **both** primary-nav surfaces, and it is never highlighted until the active-state resolver knows about it. Adding one route therefore touches four sites, and omitting any one of them is a silent gap that compiles and ships green:

- `frontend/src/components/nav/global-rail.tsx` — the desktop sidebar rail. Add a `SidebarMenuItem` `<Link>` **and** extend the `resolveActiveItem` positive-allowlist (its `ActiveItem` union *and* a matching arm). A missing arm leaves the rail item permanently un-highlighted; reuse the file's `matchesRoute(pathname, route)` helper for single-route arms (the multi-family `cardgroups` arm matching `/cards/` + `/learn/` deliberately cannot).
- `frontend/src/components/nav/logo-drawer.tsx` — the mobile drawer. Add the same `<Link>`. Wiring only the rail leaves the route undiscoverable on mobile, and vice-versa.
- `frontend/messages/{en,ja}.json` — add the `Nav.<key>` label to both catalogs (en/ja parity is required; never inline the label in the `.tsx`).
- `frontend/src/components/nav/{global-rail,logo-drawer}.test.tsx` — the canonical nav test files. Per [`scope-discipline.md`](../../.claude/rules/scope-discipline.md), the new `resolveActiveItem` arm and both new `<Link>`s become these files' responsibility: assert the `href` and (rail-only) `aria-current="page"` on the route and a sub-route, plus that the sibling item is *not* current. These tests select links by accessible-name regex, so the new label must not collide with an existing one (`/catalog/i` vs `/cardgroups/i` is safe — neither substring-matches the other).

The legacy "render `/` with health check inline" pattern is replaced by two distinct probes — see "Route Handler conventions" below. `/api/ping` is a thin liveness probe (Vercel edge reachability only, always 200) for warm-keep cron jobs; `/api/healthz` is a deep readiness probe (frontend → backend GraphQL `health`, 503 when the backend is down) for uptime monitors. External monitors that polled `/` must move to one of these — typically `/api/healthz` for alerting, `/api/ping` for warm-up that must not page on a backend outage.

The admin entry surfaces (desktop rail admin items, mobile drawer admin section) are the only UI affordances for entering `/admin`. The **middleware** (`frontend/src/lib/supabase/middleware.ts`) reads `claims.app_metadata.role` from `supabase.auth.getClaims()` (fails closed to `isAdmin=false` on any exception) and forwards `isAdmin` as the `x-user-is-admin` request header; the root layout reads that header via `readAuthContext` and passes `isAdmin` as a prop to `AppShell` (via `ConditionalShell`). The claim is emitted by a Postgres custom access token hook that joins `public.user_roles` at token mint time. `frontend/src/app/admin/layout.tsx` is the enforcement boundary — nav visibility is a UI hint, not security. See `.claude/rules/frontend-rsc-error-handling.md` for the failure-mode contract that lets the shell degrade silently when the role lookup fails.

### Global navigation primitives

The shell components live under `frontend/src/components/nav/` and compose into the root layout:

- `AppShell` — RSC; rendered from `app/layout.tsx` for every route except the bare-shell set. Owns the `<SidebarProvider>` so the desktop rail and the mobile drawer share one sidebar context. Mounts `GlobalRail` on `md+` (hidden on mobile via `hidden md:flex`) and a mobile top bar containing `LogoDrawer` on `<md`.
- `GlobalRail` — Client; the persistent desktop sidebar. Header carries the 🦩 logo (a plain `<Link href="/">`) next to a separate `SidebarToggle` icon button. Body holds Cardgroups + admin items + Settings (when signed in); footer holds Profile + sign-out. Hover-flyout opens the rail when collapsed; a `useRef`-tracked timer prevents pointer-leave flicker.
- `LogoDrawer` — Client; mobile-only. The 🦩 logo is a plain `<Link href="/">`; the `MobileMenuTrigger` is a separate adjacent button that opens a Radix `Sheet`. The drawer body groups (1) primary nav (Cardgroups, Settings, admin items), (2) Profile + sign-out below an `<hr>`.
- `SidebarToggle` / `MobileMenuTrigger` — Client atoms. The logo is split from the toggle on both surfaces: tapping 🦩 always navigates home; the adjacent button always opens/closes the nav. Conflating "go home" and "expand the rail" onto one element is a design-systems anti-pattern — a tap on the logo would either do the wrong thing for half the user's intents or silently change behaviour based on collapsed/expanded state.
- `LogoDrawer` "+" — Client; the single nav-header "+" in the mobile top bar. Rendered only when `resolveHeaderCreateAction(pathname)` returns a non-null action **and** the user is signed in (anonymous users get no "+"). On click it dispatches a cancelable `flamingo:add-cardgroup` (for the `cardgroup` kind) or `flamingo:add-card` (for the `card-with-group` kind) `CustomEvent`: an in-context drawer mounted on the page can claim the action via `preventDefault`, otherwise the handler falls back to the full-page route via `router.push`. The `role` kind has no `href` and instead opens the create sheet by writing `?new=true` through `useSheetSearchParam`. Because the target is derived solely from the pathname, the "+" never pre-pends a `?cardgroup=...` on a generic route — only the edit / learn variants build that query (`/cards/new` still owns the 4-priority resolution).

#### Breakpoint-exclusive primary action

When two surfaces would render the same primary CTA at the same time (for example, a desktop page-shell header action and a breakpoint-scoped affordance), make them **mutually exclusive by Tailwind class** rather than by a JS `useMediaQuery` hook. The surviving pair is the desktop page-shell header action (`hidden md:inline-flex`, e.g. `cardgroups-header-new-btn` in `cardgroups-client.tsx`) and the breakpoint-agnostic empty-state CTA — the desktop header button is hidden below `md`, so it never doubles with the mobile-bar affordances. Two reasons to do this in CSS: (1) SSR and CSR render the same markup, so there is no hydration flash where both affordances briefly appear; (2) no React re-render fires on viewport change — the browser handles the transition in CSS. Reach for `useMediaQuery` only when the *content* (not just visibility) differs across breakpoints, which is rare.

#### Current-location CTA — `aria-current="page"` plus disabled styling

When a primary CTA's destination is the page the user is already on, do not hide it (the layout would jump and the user loses orientation) and do not disable it as a `<button>` (it is a `<Link>`, not a button). Apply `aria-current="page"` for screen-reader semantics and pair with a visually-subdued, non-navigating treatment for the same `<Link>`. Drop the hover variant on the same branch — a hover-color flash on a non-interactive element is dishonest. `GlobalRail` (`frontend/src/components/nav/global-rail.tsx`) is the reference implementation: its Cardgroups, admin, and Profile links set `aria-current={isActive ? "page" : undefined}` and surface the active treatment through the shadcn `SidebarMenuButton isActive={...}` prop.

#### Null-default single resolver: no separate hide-regex, no suppression context

The "should a '+' exist on this route" decision is folded into the one positive resolver `resolveHeaderCreateAction(pathname)` — it returns a typed action for the four matched routes and `null` for everything else. There is no companion hide-regex and no suppression context: "no route match → `null` → no '+' rendered" is the whole rule. The resolver matches in order (`/cardgroups` exact, then the `/cardgroups/:id/edit` and `/learn/:id` regexes, then `/admin/roles` exact); the trailing `return null` covers every other path.

Collapsing the decision into a single positive resolver structurally eliminates the cross-layer shadow bug-class that a static-hide-list plus a separate action-resolver would carry. There is no path that one layer hides while another layer still produces an action for it, because there is only one layer. Concretely, `resolveHeaderCreateAction("/cardgroups/new")` returns `null` directly — it is not the exact string `/cardgroups`, and it matches neither the edit nor the learn regex — so the new-cardgroup form can never accidentally surface a "+" pointing back at itself.

### Cardgroup chip + picker primitives

`CardgroupChip` (client) and `CardgroupPickerSheet` (client) live under `frontend/src/components/cardgroups/`. The chip is a short pill with the current cardgroup name + `ChevronDown`; tapping it opens the picker. The picker is a shadcn `Sheet` rendered with `side="bottom"` on all viewport sizes (the original plan considered a desktop `Dialog` variant via `useMediaQuery`, but a bottom sheet works on both and avoids dragging in a media-query hook just for one component). It is backed by `MyCardgroupsConnectionQuery` via `useQuery`. Two consumers exist today: `/cards/new` (chip + form pair, single source of truth in URL) and any future page that needs cardgroup-scoped writes from a non-cardgroup-scoped route.

The chip truncates names to `max-w-[12ch] sm:max-w-[20ch]`; long names show as ellipsis-on-mobile and the full name appears in the picker. There is no tooltip — tapping the chip already reveals the full list.

### Cardgroup creation flow — two entry points

Cardgroup creation entry points are intentionally asymmetric because card and cardgroup creation frequencies are roughly 95:5. Two surfaces initiate cardgroup creation:

1. **From `/cardgroups` list** (page-shell `primaryActions` slot): The "+ New cardgroup" link is rendered in the `<ListingPageShell>` header `primaryActions` cluster — visible in BOTH the populated and empty-state branches of the list. Clicking it navigates to `/cardgroups/new` without a `returnTo` query, and the completion page is `/cardgroups/{newId}` — the new cardgroup's detail page. The earlier dashed empty-state CTA and the populated-state footer link were both consolidated into this single shell-level affordance, so the empty state and the populated state surface exactly one create entry point at the same screen position.
2. **From `/cards/new` picker** (always-visible inline link): The `CardgroupPickerSheet` always displays a "+ Create new cardgroup…" link at the bottom, even when cardgroups exist. Clicking it navigates to `/cardgroups/new?returnTo=/cards/new`, embedding the return destination into the query parameter. After creation, the page redirects to `/cards/new?cardgroup={newId}` with the new cardgroup pre-selected in the form, keeping the user in the card creation flow without a detour.

**Open-redirect guard**: The `returnTo` query is sanitized server-side in `frontend/src/app/cardgroups/new/page.tsx` via the `sanitizeReturnTo` function. Only paths starting with `/` (and not `//` or `/\`) are accepted; everything else is rejected and defaults to `/cardgroups/{newId}`. The `/\` rejection blocks the browser-normalised backslash bypass — Chrome and Firefox rewrite `/\evil.com` to `//evil.com` and follow the protocol-relative URL off-domain.

The asymmetry reflects the information hierarchy: the nav-header "+" and the per-page add-card affordances remain the global, frequent path; cardgroup creation is a setup-level operation accessible only when creating a card (picker) or managing the cardgroup list (/cardgroups). This surfaces the card → cardgroup parent-child relationship in the interaction flow.

### Design tokens — brand palette usage rules

`globals.css` defines five brand tokens (`--brand-primary`, `--brand-primary-foreground`, `--brand-tint`, `--brand-tint-border`, `--brand-tint-foreground`). The product palette is intentionally narrow:

- `--brand-primary` (coral `#FE7F70` via OKLCH) — primary CTAs only: primary form Save buttons and the sonner toast `actionButton` (the Undo button on delete toasts). Not for body text, links, or hover states. The sonner `actionButton` is included because it is the sole recovery affordance for a committed delete — it must be unambiguously distinct from the toast body and the `cancelButton`. Apply via `classNames.actionButton: "group-[.toast]:bg-brand-primary group-[.toast]:text-brand-primary-foreground"` in the `<Toaster>` component options.
- `--brand-tint*` — low-volume brand-identity surfaces: (1) the sign-in brand panel (`/login` left column), (2) admin entry surfaces (rail admin items, the drawer admin section). The tint signals "this surface carries product identity or marks a context switch" without competing with a primary CTA. Use `bg-brand-tint` for the panel background, `text-brand-tint-foreground` for headings, and `text-brand-tint-foreground/80` for secondary copy on that background.

Every other surface uses shadcn's slate-based defaults (`--primary`, `--secondary`, `--accent`). New components should only reach for the brand tokens when they fall into one of the categories above — adding a fourth use site for `--brand-tint*` or a second category for `--brand-primary` dilutes the signal, so reviewers should push back unless the new surface is clearly identity-bearing or a primary CTA.

### Welcome copy on `/cardgroups/new?welcome=1`

The `?welcome=1` query parameter makes `/cardgroups/new` (already the cardgroup-create page) double as the post-onboarding "first cardgroup" screen by conditionally rendering a welcome banner above the form. The `/cards/new` "no cardgroups" priority targets this URL directly, as does `/onboarding/start` — via its empty-catalog fallback (zero published masters) and via its "start with the default decks" action when zero default starters were seeded (`count === 0`). The HomePage `else` branch and `OnboardingForm`'s success redirect route through `/onboarding/start` first, landing here when the catalog is empty or when the default-starter seed produced no decks. Without the query parameter, the page renders only the form — same behaviour as before. Driving the difference from the URL keeps the welcome surface statelessly bookmarkable / sharable and avoids a separate `/welcome` route whose only difference would be the copy.

### Bare-shell routes (no `AppShell`)

`ConditionalShell` (`frontend/src/components/conditional-shell.tsx`) gates `AppShell` with a content-route **allowlist** (`SHELL_ROUTE_PREFIXES` — `/admin`, `/cardgroups`, `/cards`, `/catalog`, `/learn`, `/profile`): the nav shell mounts only on those in-app sections, and every other path renders bare. The bare set therefore covers `/`, `/login`, `/onboarding`, `/onboarding/start`, `/terms`, `/privacy`, and — by construction — any unknown path. Most bare pages are full-screen and own the entire viewport — `/login` because the sign-in screen owns the viewport; `/onboarding` because the user has no `displayName` yet, so a header showing their email next to an empty name slot would surface the very state the page is asking them to fix; `/onboarding/start` because a deckless user's nav rail would point at empty destinations, and it is part of the same focused onboarding flow; `/terms` and `/privacy` because they are public legal pages. `/` is the redirect-only dispatcher (`app/page.tsx` always `redirect()`s — see the HomePage redirect chain above) that never renders content; mounting the shell there only flashes the nav rail for the brief moment `/` resolves and redirects to the user's real destination — most visibly on a new user's first login (`/` → `/onboarding`), where the left rail appears then vanishes. An unknown path renders `app/not-found.tsx` (404) under the arbitrary pathname the user typed; the allowlist makes that 404 bare too, so a not-yet-onboarded user who mistypes a URL never sees the rail. (A 404 raised *under* a content prefix — e.g. `/learn/<bad-id>` — keeps the shell intentionally: that caller is onboarded and mistyped a real section. 500-class errors are unaffected — `app/error.tsx` paints a full-screen splash over any shell and `app/global-error.tsx` replaces the root layout.) Every bare route *that renders content* emits its own `<main>` landmark — see [`rsc-error-handling/pages-bypassing-appshell-must-render-main.md`](./rsc-error-handling/pages-bypassing-appshell-must-render-main.md); `/` is exempt because it renders no content of its own, only the transient `app/loading.tsx` splash (a `role="status"` region) before it redirects.

### Sign-in page layout (`/login`)

`/login` uses a split-screen shell:

```tsx
<main data-testid="login-grid" className="relative grid h-svh lg:grid-cols-2">
  <div className="max-lg:hidden ... bg-brand-tint">{/* brand panel */}</div>
  <div className="flex flex-col">{/* form column: h1, error banner, OAuth button, footer */}</div>
</main>
```

Three rules apply to any future page that adopts this shell (e.g. `/signup`, `/reset-password`):

1. **`h-svh`, not `h-screen` or `min-h-screen`.** `h-screen` resolves to `100vh`, which on Safari mobile includes the address-bar height that collapses on scroll — a non-scrolling full-viewport page measured against `100vh` ends up taller than the visible area and the bottom content is cut off. `h-svh` (small viewport height) is the always-visible height and is the correct unit for a fixed-viewport login shell. Use `min-h-svh` only when the content can grow taller than the viewport.
2. **Brand panel hidden below `lg` with `max-lg:hidden`.** The left column hides below `lg`; the right column is always visible. This keeps the mobile experience a single-column form (no wasted vertical space for branding) while letting desktop carry the full-bleed brand panel.
3. **`<main>` landmark required.** Same as every bare-shell route — see "Bare-shell routes" above.

The Terms / Privacy footer block is wrapped in `<footer>` rather than `<p>` for semantic clarity, though ARIA spec only exposes `<footer>` as the `contentinfo` landmark when it is a direct child of `<body>`. Nested inside `<main>` here, browsers expose it as a generic group, not a landmark; the `<footer>` choice is therefore semantic improvement without a landmark-navigation gain.

