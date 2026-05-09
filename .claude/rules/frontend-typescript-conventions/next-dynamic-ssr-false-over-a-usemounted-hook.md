# `next/dynamic({ ssr: false })` over a `useMounted` hook for hydration-sensitive client-only components

> Part of [`.claude/rules/frontend-typescript-conventions.md`](../frontend-typescript-conventions.md). See the index for related rules.

A component that wraps a client-only library (`@react-spring/web`, `@use-gesture/react`, libraries that touch `window` at module scope, animation libraries that paint on first frame) needs a way to skip server-side rendering without a hydration mismatch. The historic shape was `useMounted` — a `useState(false)` plus `useEffect(() => setMounted(true), [])` that gates the client-only render branch. The recommended shape is `dynamic(() => import("./animated-card").then((m) => m.AnimatedCard), { ssr: false })`: the dynamic import handles the SSR skip at the module-loading boundary, eliminating the double render and the `useMounted` state entirely.

```tsx
// frontend/src/components/learn/swipe-card.tsx
import dynamic from "next/dynamic";

// AnimatedCard ships @react-spring/web + @use-gesture/react, which both
// require client-only execution. We load it via next/dynamic (ssr: false)
// to avoid the SSR/hydration mismatch that previously needed useMounted.
//
// Trade-off: a chunk-load failure (network blip after deploy, CDN miss)
// renders the loading fallback (`null`) without surfacing an error UI.
// The component area is briefly blank and the user must navigate away to
// recover. Accepted because: (a) chunk failures are rare in production,
// (b) the surrounding flow is forgiving, (c) adding an error fallback
// complicates the success path's rendering for an edge case. Revisit if
// telemetry shows non-trivial chunk-failure rates on the affected route.
const AnimatedCard = dynamic(() => import("./animated-card").then((m) => m.AnimatedCard), {
  ssr: false,
});
```

**Why:** `useMounted` is the canonical "you might not need an effect" anti-pattern — the effect's only job is to flip a flag, the flag's only job is to gate the render branch, and the flag exists only because the component is unsafe to render on the server. `next/dynamic` solves the same problem at the module-loading boundary so the consuming component does not need a flag at all. The double render that `useMounted` produces (first pass with the static-fallback branch, second pass with the animated branch) is also a hydration-risk band-aid: if the static fallback's DOM differs in attributes from the animated component's first paint, React still warns about a mismatch on the post-mount render. `next/dynamic` skips the server render entirely, so there is no hydration to mismatch.

**Trade-off — chunk-load failure has no error UI by default.** A `dynamic` import that fails (transient network error, CDN miss after a deploy) renders the loading fallback (`null` by default, or the `loading` callback if provided) and stays there. There is no `error` boundary callback exposed by `next/dynamic`'s API. Document the trade-off in a code comment at the call site so a future contributor does not assume the absence of an error fallback was an oversight. The acceptance criteria for skipping the error fallback are: (1) chunk failures are rare in production, (2) the surrounding flow has a graceful out (the user can navigate away or reload), and (3) adding the error UI would complicate the success path. If telemetry surfaces a non-trivial chunk-failure rate on a specific route, revisit by wrapping the dynamic component in an error boundary with a route-specific fallback.

**How to apply:** any component that previously used `useMounted` (or any equivalent `useState(false) + useEffect(() => setX(true), [])` hydration-skip pattern) MUST migrate to `next/dynamic({ ssr: false })` and document the chunk-load trade-off in a code comment. Do not introduce new `useMounted` hooks. Reference: `frontend/src/components/learn/swipe-card.tsx` (`AnimatedCard` via `next/dynamic`) replaced a prior `useMounted` gate. This rule pairs with § "`useSyncExternalStore` over `useState + useEffect`" above: both delete a `useState + useEffect` initialization pattern in favour of a primitive that React or Next.js provides for the exact use case.
