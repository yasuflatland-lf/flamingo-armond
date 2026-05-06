# feat(login): Split-Screen Sign-In Layout — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the single-column `/login` page with a `lg:grid-cols-2` split-screen — brand panel on the left (hidden below `lg`), sign-in form on the right (always visible).

**Architecture:** `LoginPage` stays a server component. A single `<div data-testid="login-grid" className="relative grid h-svh lg:grid-cols-2">` wrapper holds two child columns. Left column uses `max-lg:hidden`; right column is always rendered. No new client components or sub-files introduced.

**Tech Stack:** Next.js 15 App Router (RSC), Tailwind CSS v4, Vitest + Testing Library

---

## Progress

- [x] Plan written — 2026-05-06
- [x] Task 1A: New tests written
- [x] Task 1B: Split-screen layout implemented
- [x] Task 2: Tests verified, typecheck/lint pass
- [x] Task 3: Committed (38acf25 → cherry-picked as 42f8099 onto feature/uiux_improvement_20240506)
- [x] Step 2 (round 1): Critical fixed — `<main>` landmark; duplicate comment removed; 3 tests added (commit e8192b4)
- [x] Step 2 (round 2): PASS — no remaining Critical/Important issues
- [x] Step 3: Code simplified — JSX comments removed, tests consolidated with nested describe+beforeEach (commit 6a0bf0c)
- [x] Step 4: Full test suite verified — 535/535 pass, typecheck clean
- [x] Step 5: Docs updated — h-svh rule, <main> landmark rule, brand-tint widened (commit ecff7a5)

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `frontend/src/app/login/page.tsx` | Modify | Replace `<main>` shell with split-screen grid |
| `frontend/src/app/login/page.test.tsx` | Modify | Add responsive-class and brand-panel assertions |

## Parallelism

- **Cluster A** (Task 1A — runs in parallel with Cluster B): Add new test cases to `page.test.tsx`
- **Cluster B** (Task 1B — runs in parallel with Cluster A): Implement split-screen in `page.tsx`
- **Cluster C** (Task 2 — depends on A + B completing): Run tests, typecheck, lint
- **Cluster D** (Task 3 — depends on C passing): Commit via dedicated commit agent

---

## Task 1A: Add split-screen test cases to page.test.tsx

**Files:**
- Modify: `frontend/src/app/login/page.test.tsx`

- [ ] **Step 1: Add 5 new tests inside the existing `describe("LoginPage", ...)` block (after the last existing test)**

Append after line 101 (the `});` closing the last `it(...)` block), before the final `});` of the describe:

```tsx
  it("split-screen: outer wrapper has h-svh and lg:grid-cols-2", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(makeSupabaseMock(null) as never);

    const jsx = await LoginPage({ searchParams: Promise.resolve({}) });
    const { container } = render(jsx);

    const grid = container.querySelector("[data-testid='login-grid']");
    expect(grid).toBeInTheDocument();
    expect(grid?.className).toMatch(/h-svh/);
    expect(grid?.className).toMatch(/lg:grid-cols-2/);
  });

  it("split-screen: brand panel has max-lg:hidden", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(makeSupabaseMock(null) as never);

    const jsx = await LoginPage({ searchParams: Promise.resolve({}) });
    const { container } = render(jsx);

    const brandPanel = container.querySelector("[data-testid='brand-panel']");
    expect(brandPanel).toBeInTheDocument();
    expect(brandPanel?.className).toMatch(/max-lg:hidden/);
  });

  it("split-screen: brand panel shows logo, app name, and value copy", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(makeSupabaseMock(null) as never);

    const jsx = await LoginPage({ searchParams: Promise.resolve({}) });
    const { container } = render(jsx);

    const brandPanel = container.querySelector("[data-testid='brand-panel']");
    expect(brandPanel).toBeInTheDocument();
    expect(brandPanel?.querySelector("[aria-label='Flamingo']")).toBeInTheDocument();
    expect(brandPanel?.textContent).toContain("flamingo-armond");
  });

  it("split-screen: brand panel must not contain any email address (PII)", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(makeSupabaseMock(null) as never);

    const jsx = await LoginPage({ searchParams: Promise.resolve({}) });
    const { container } = render(jsx);

    const brandPanel = container.querySelector("[data-testid='brand-panel']");
    expect(brandPanel?.textContent).not.toMatch(/@/);
  });

  it("split-screen: form column renders OAuth button, Terms link, and Privacy link", async () => {
    vi.mocked(createSupabaseServerClient).mockResolvedValue(makeSupabaseMock(null) as never);

    const jsx = await LoginPage({ searchParams: Promise.resolve({}) });
    render(jsx);

    expect(screen.getByRole("button", { name: /sign in/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /terms/i })).toHaveAttribute("href", "/terms");
    expect(screen.getByRole("link", { name: /privacy/i })).toHaveAttribute("href", "/privacy");
  });
```

- [ ] **Step 2: Confirm new tests fail before implementation**

Run: `cd frontend && pnpm test --run src/app/login/page.test.tsx 2>&1 | tail -30`

Expected: 5 new tests FAIL (data-testid `login-grid` and `brand-panel` not found yet). 5 original tests still PASS.

---

## Task 1B: Implement split-screen layout in page.tsx

**Files:**
- Modify: `frontend/src/app/login/page.tsx`

- [ ] **Step 1: Replace lines 20–28 (the `return (...)` block) with the split-screen layout**

The final file must be exactly:

```tsx
import { redirect } from "next/navigation";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { LoginButton } from "./login-button";

type SearchParams = Promise<{ error?: string }>;

export default async function LoginPage({ searchParams }: { searchParams: SearchParams }) {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();
  if (authErr && authErr.name !== "AuthSessionMissingError") {
    console.error("[login] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (user) redirect("/cardgroups");

  const { error } = await searchParams;
  return (
    <div data-testid="login-grid" className="relative grid h-svh lg:grid-cols-2">
      {/* Brand panel — hidden below lg breakpoint */}
      <div
        data-testid="brand-panel"
        className="max-lg:hidden flex flex-col items-center justify-center gap-6 bg-brand-tint"
      >
        <span className="text-7xl" role="img" aria-label="Flamingo">
          🦩
        </span>
        <div className="flex flex-col items-center gap-1 text-center">
          <span className="text-2xl font-semibold text-brand-tint-foreground">
            flamingo-armond
          </span>
          <span className="text-sm text-brand-tint-foreground/80">
            Remember more, study less.
          </span>
        </div>
      </div>

      {/* Form column — always visible */}
      <div className="flex flex-col">
        <div className="flex flex-1 flex-col items-center justify-center gap-4 p-8">
          <h1 className="text-2xl font-semibold">Sign in</h1>
          {error ? (
            <p className="text-sm text-destructive">Sign-in failed: {error}</p>
          ) : null}
          <LoginButton />
        </div>
        <footer className="pb-6 px-8 text-center text-xs text-muted-foreground">
          By signing in, you agree to our{" "}
          <a href="/terms" className="underline underline-offset-2 hover:text-foreground">
            Terms of Service
          </a>{" "}
          and{" "}
          <a href="/privacy" className="underline underline-offset-2 hover:text-foreground">
            Privacy Policy
          </a>
          .
        </footer>
      </div>
    </div>
  );
}
```

---

## Task 2: Verify tests, typecheck, lint (depends on 1A + 1B)

- [ ] **Step 1: Run login page tests**

Run: `cd frontend && pnpm test --run src/app/login/page.test.tsx 2>&1 | tail -30`

Expected: All 10 tests PASS.

- [ ] **Step 2: Run full test suite**

Run: `cd frontend && pnpm test --run 2>&1 | tail -20`

Expected: All tests pass. No regressions.

- [ ] **Step 3: Typecheck and lint**

Run: `cd frontend && pnpm typecheck 2>&1 | tail -10 && pnpm lint 2>&1 | tail -10`

Expected: Both exit 0.

- [ ] **Step 4: Language policy check**

Run: `grep -rP "[\x{3040}-\x{30ff}\x{4e00}-\x{9fff}]" frontend/src/app/login/ 2>&1`

Expected: No output (no CJK characters).

---

## Task 3: Commit (dedicated commit agent — depends on Task 2)

- [ ] **Step 1: Stage changed files**

```bash
git add frontend/src/app/login/page.tsx frontend/src/app/login/page.test.tsx
```

- [ ] **Step 2: Commit**

```bash
git commit -m "feat(login): split-screen brand panel and form layout"
```

---

## Self-Review Notes

- All 8 acceptance criteria from issue #99 are covered by Tasks 1A/1B/2.
- `h-svh` (not `min-h-screen`) is used for correct Safari dynamic viewport handling.
- `max-lg:hidden` hides the brand panel below `lg` breakpoint.
- Error query param handling and redirect preserved exactly.
- No CJK in any new strings.
- Terms/Privacy footer uses placeholder hrefs (`/terms`, `/privacy`).
- `data-testid` attributes enable reliable test selectors without CSS-class brittleness.
