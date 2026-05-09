# Radix `asChild` Slot collapses a `null` child into an empty wrapper — gate at the parent

> Part of [`.claude/rules/frontend-typescript-conventions.md`](../frontend-typescript-conventions.md). See the index for related rules.

Radix UI primitives that accept `asChild` (e.g. `<Sheet>`, `<SidebarMenuButton>`, `<TooltipTrigger>`, `<SheetClose>`) forward props to the rendered child via the Radix `Slot` component. When the child component returns `null` (e.g. a self-suppressing `<HeaderSignInLink>` that returns `null` on `/login`), `Slot` renders nothing — but the **wrapping** Radix container (`<SidebarFooter>`, the `<nav>` block, the `<SidebarMenuItem>`) is still mounted, leaving an empty rectangle in the layout with the wrapper's padding, border, and ARIA semantics intact. There is no DOM-level signal that the slot collapsed; CSS-only review misses it because the empty container is a 1-pixel-tall gap.

```tsx
// AVOID: child self-suppresses on /login, but the SidebarFooter still mounts.
{user === null && (
  <SidebarFooter>
    <SidebarMenu>
      <SidebarMenuItem>
        <SidebarMenuButton asChild tooltip="Sign in">
          <HeaderSignInLink />  {/* returns null on /login */}
        </SidebarMenuButton>
      </SidebarMenuItem>
    </SidebarMenu>
  </SidebarFooter>
)}

// PREFER: gate the wrapper at the same place the child would self-suppress.
{user === null && pathname !== "/login" && (
  <SidebarFooter>
    {/* ...same child tree... */}
  </SidebarFooter>
)}
```

**Why:** the child's self-suppression is correct for the standalone case (e.g. when `<HeaderSignInLink>` is rendered inside something that does not have its own padding), but `asChild` Slot composition does not propagate "child rendered nothing" up to the wrapper. The two layers must agree on the suppression condition, or the layer with the broader visibility wins by default. Self-suppression in the child is convenient for one-off use; gating at the parent is correct when the parent contributes its own visual chrome.

**How to apply:** any time a child of a Radix `asChild` slot returns `null` for a known input, audit every wrapper in the chain that contributes visible chrome (padding, border, `<hr>`, ARIA landmark) and gate the outermost contributor on the same condition. Pair the parent gate with a co-located test that mounts the parent on the suppressing route and asserts the wrapper itself is absent — `expect(screen.queryByRole("contentinfo")).not.toBeInTheDocument()` is more forcing than `queryByRole("link", { name: /Sign in/i })` because the link is gone in both the right and wrong implementations. Reference: `frontend/src/components/nav/global-rail.tsx` and `frontend/src/components/nav/logo-drawer.tsx` (`pathname !== "/login"` gate around the Sign-in `SidebarFooter` / drawer `<nav>`).
