# shadcn/ui

> Part of [`docs/frontend.md`](../frontend.md). See the index for related chapters.

`frontend/components.json` and `frontend/src/lib/utils.ts` (the `cn()` helper) are committed. The initial component set (`button`, `input`, `label`) was added with `pnpm dlx shadcn add` and extends `globals.css` with the required theme tokens. `form.tsx` was removed (TanStack Form's render-prop API does not need the shadcn wrapper); `textarea.tsx` was added for the `bio` field. `alert-dialog` and `dialog` primitives are now installed — use `AlertDialog` for destructive confirms (delete), `Dialog` for non-destructive overlays.

`shadcn init` is interactive and not suitable for CI or non-interactive environments. The fallback is to hand-write `components.json`, `lib/utils.ts`, and the `globals.css` base tokens following the shadcn JSON schema — that is how this repo's shadcn baseline was bootstrapped.

