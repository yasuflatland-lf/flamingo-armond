# Widening a prop from `string` to `string | ReactNode` breaks string-concatenation fallbacks

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

When a shared component's prop type is widened from `string` to `string | React.ReactNode`, TypeScript allows the change at every call site — the type-check passes silently. However, any code inside the component that constructs a string from the prop using template literals or string concatenation will coerce a non-string `ReactNode` to `"[object Object]"`:

```ts
// When title is a ReactNode (e.g. a <span>), this produces "[object Object] form"
const a11yDescription = `${title} form`;
```

The resulting computed value is not a useful accessible description, and the bug only surfaces at runtime.

## The fix

Guard with a `typeof` check before the template literal:

```ts
const a11yDescription =
  description ?? (typeof title === "string" ? `${title} form` : "Form");
```

When `title` is a `string`, the template literal is safe. When it is a `ReactNode`, fall back to a generic description (`"Form"`) or accept the explicit `description` prop.

## Why the type-check alone does not catch this

`ReactNode` includes `string` as one of its members. From the TypeScript compiler's perspective, `${title}` is valid when `title: string | ReactNode` because the template literal calls `.toString()` on its operands. The compiler trusts that `.toString()` returns a useful value; it does not know that a React element's `.toString()` produces `"[object Object]"`. The bug is a semantic trap, not a type error.

## Testing

The fallback behavior is the correct assertion target. Verify that a component receiving a `ReactNode` title produces a readable accessible description:

```ts
it("a11yDescription falls back to 'Form' when title is a ReactNode and no description is provided", () => {
  render(
    <FormSheet open onOpenChange={vi.fn()} title={<span>Some ReactNode title</span>}>
      <p>Body</p>
    </FormSheet>,
  );

  expect(screen.getByRole("dialog")).toHaveAccessibleDescription("Form");
});
```

An assertion against `toHaveAccessibleDescription("[object Object] form")` would catch the broken state, but the test should assert the correct value — if it passes with `"[object Object] form"`, the fallback is still broken.

Reference: `frontend/src/components/ui/form-sheet.tsx` — `a11yDescription` derivation on line 76; `frontend/src/components/ui/form-sheet.test.tsx` — `"a11yDescription falls back to 'Form'"` test case.
