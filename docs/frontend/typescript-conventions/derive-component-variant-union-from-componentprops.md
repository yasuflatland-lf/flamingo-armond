# Derive a shadcn component's variant union via `ComponentProps`, not a hand-written copy

> Applies to: any `frontend/src/**` consumer that maps domain values to a shadcn component's `variant` / `size` prop.

A consumer that maps a domain enum to a shadcn `Badge` variant needs a value type for the map. The tempting shortcut is to hand-write the subset of variants used:

```ts
const VARIANT: Record<MasteryStage, "outline" | "secondary" | "default"> = { ... };
```

**Why this drifts.** The shadcn `Badge` (`@/components/ui/badge`) exports only the `Badge` component — its props interface is not exported. The real variant union (`"default" | "secondary" | "destructive" | "outline"`) is derived from the `cva` config and lives only inside `badge.tsx`. A hand-written copy in a consumer has no structural tie to it: if a future shadcn upgrade renames a variant (e.g. `"default"` → `"primary"`), TypeScript cannot flag the now-stale string, and `cva` silently applies no class for an unknown variant at runtime — no error, no style.

**The fix.** Derive the type from the component itself, even though the props interface is unexported:

```ts
import type { ComponentProps } from "react";
import { Badge } from "@/components/ui/badge";

type BadgeVariant = NonNullable<ComponentProps<typeof Badge>["variant"]>;
const VARIANT: Record<MasteryStage, BadgeVariant> = { ... };
```

`ComponentProps<typeof Badge>` extracts the props type from the component value; `["variant"]` selects the variant union; `NonNullable<…>` trims the `null | undefined` that `VariantProps` includes. A future rename now fails to compile at the `VARIANT` declaration.

**Boundary.** Use this whenever a consumer maps to a shadcn component's `variant`/`size` prop. If the component does export its props type, importing that is equally fine — the rule is "never hand-copy the union," not "always use `ComponentProps`." The `Record<Domain, BadgeVariant>` exhaustiveness check (a compile error when a domain key is missing) is a separate guarantee and is unaffected.

Worked example: `frontend/src/components/learn/mastery-badge.tsx`.
