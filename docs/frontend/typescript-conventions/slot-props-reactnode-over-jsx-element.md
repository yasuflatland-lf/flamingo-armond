# Slot/content props use `ReactNode`, not `JSX.Element`

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

A component that accepts content or children from its caller should type those props as `ReactNode`, not `JSX.Element`. The distinction matters because `JSX.Element` is narrower than what the React runtime actually accepts for a slot.

```tsx
// AVOID — rejects cond && <X/>, arrays, strings
function Footer(props: { left?: JSX.Element; right: JSX.Element }): JSX.Element { ... }

// PREFER — accepts every valid child the runtime renders
function Footer(props: { left?: ReactNode; right: ReactNode }): JSX.Element { ... }
```

**Why:** `JSX.Element` is the type of a single JSX expression evaluated at call time. It excludes several React child shapes the runtime renders without issue: `cond && <X />` (whose type includes `false`), arrays of elements, plain strings and numbers, and fragments. Callers that pass any of these shapes produce a TypeScript error even though the output is correct at runtime. `ReactNode` is the canonical type for "anything React can render" and matches the actual slot contract. This convention is established throughout the codebase — `frontend/src/components/layout/listing-page-shell.tsx` types every slot (`title`, `description`, `primaryActions`, `toolbar`, `children`) as `ReactNode`.

The required-vs-optional split on slot props is orthogonal and should reflect the design contract, not the type choice: `right: ReactNode` (required, primary content always present) and `left?: ReactNode` (optional, secondary content may be absent). Both slots remain `ReactNode` regardless of their optionality. Reference: `WizardFooter` in `frontend/src/components/cardgroups/cardgroup-batch-import-form.tsx` uses exactly this split while declaring `: JSX.Element` as its return type.

**How to apply:** this rule applies to **slot and content props** only — the props a caller passes in. It does not apply to a function component's return-type annotation. Returning `: JSX.Element` from a component is a separate, well-established convention in this codebase (a component always returns a single, non-null element) and is out of scope for this rule. Do not change return-type annotations in response to this rule. When adding or reviewing a component's props interface, replace any `JSX.Element` or `JSX.Element | null` prop type that represents a caller-supplied slot with `ReactNode`; leave `?: ReactNode` for optional slots and `ReactNode` (required) for slots that must always be provided.
