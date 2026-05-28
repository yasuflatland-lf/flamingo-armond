# Test the ARIA contract via `toHaveAccessibleName`/`toHaveAccessibleDescription`, not `textContent`

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

`heading.textContent` and `expect(document.body.textContent).toContain("Form")` pass incidentally when text is present, but they do not verify that the `aria-labelledby` or `aria-describedby` wiring is correct. A component whose heading text happens to include the expected string will pass these assertions even if the `aria-labelledby` attribute points at the wrong element or is missing entirely — the ARIA relationship exists only in the attribute, not in the raw text.

## Prefer role-based accessiblity queries

Assert the computed accessible name and description on the interactive role element directly:

```ts
// Correct: verifies the aria-labelledby wiring between the dialog and its title
expect(screen.getByRole("dialog")).toHaveAccessibleName(/batch import into.*cardgroup name/i);

// Correct: verifies the aria-describedby wiring
expect(screen.getByRole("dialog")).toHaveAccessibleDescription("Edit card form");

// Incorrect: passes incidentally; does not verify aria-labelledby is set
expect(screen.getByRole("heading").textContent).toContain("Edit card");

// Incorrect: passes incidentally; does not verify the description element is wired
expect(document.body.textContent).toContain("Form");
```

`toHaveAccessibleName` and `toHaveAccessibleDescription` from `@testing-library/jest-dom` call the browser's accessibility tree computation (via `aria-query` + computed role resolution) and verify the string that AT would actually announce.

## Parametrize over both branches when a component renders two paths

`FormSheet` renders a Radix `Sheet` on desktop and a `vaul` Drawer on mobile, both accepting the same `title` prop. If both paths were changed identically, a test that mocks `useIsMobile` as `false` only covers the Radix path. Use `describe.each` to toggle the mock and cover both:

```ts
describe.each([{ mobile: false }, { mobile: true }])(
  "ReactNode title — mobile=$mobile",
  ({ mobile }) => {
    beforeEach(() => {
      vi.mocked(useIsMobile).mockReturnValue(mobile);
    });

    it("exposes both verb and destination in the accessible name", () => {
      // ...
      expect(screen.getByRole("dialog")).toHaveAccessibleName(/batch import into/i);
    });
  },
);
```

This pattern applies to any component that selects between two structurally identical UI paths based on a feature flag, viewport check, or user preference. A single-branch test leaves the other branch uncovered and gives false confidence that the ARIA contract holds uniformly.

Reference: `frontend/src/components/ui/form-sheet.test.tsx` — `describe.each([{ mobile: false }, { mobile: true }])` block; `"renders a string-title desktop sheet with accessible description '<title> form'"` test case using `toHaveAccessibleDescription`.
