# Pages that bypass `AppShell` MUST render their own `<main>` landmark

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

The root layout in `frontend/src/app/layout.tsx` short-circuits `AppShell` for any route that owns the full viewport — the bypass set lives in a literal-equality check against `pathname`. When `AppShell` is bypassed, the rendered tree contains no `<main>`, no `<nav>`, and no shell-level landmarks — the page itself is the only place a landmark can be emitted. Without an explicit `<main>`, screen readers (VoiceOver, JAWS, NVDA) have no jump-to-content target and the page fails WCAG 2.1 SC 1.3.6 ("Identify Purpose"). For the canonical list of bare-shell routes and the design rationale, see [`docs/frontend/routing-topology.md` § "Bare-shell routes (no `AppShell`)"](../routing-topology.md#bare-shell-routes-no-appshell).

```tsx
// frontend/src/app/login/page.tsx
return (
  <main data-testid="login-grid" className="relative grid h-svh lg:grid-cols-2">
    {/* page content */}
  </main>
);
```

**How to apply:** any page added to the `AppShell`-bypass branch of `app/layout.tsx` MUST render `<main>` as its outermost content wrapper. Co-locate a test that asserts `screen.getByRole("main")` resolves on the bypassed route — the assertion throws when the landmark is absent, so it is forcing rather than tautological. The shell-rendered routes do not need this rule because `AppShell` already emits the landmark at the layout level; a second `<main>` in a child page would create duplicate landmarks and confuse assistive technology.
