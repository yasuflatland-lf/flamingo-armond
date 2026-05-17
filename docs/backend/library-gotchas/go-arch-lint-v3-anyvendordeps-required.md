# go-arch-lint v3: every `deps` entry must declare at least one permission flag

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

The v3 archfile spec validator requires every entry under the `deps:` block to
declare **at least one** of these permission flags:
`mayDependOn`, `canUse`, `anyProjectDeps`, or `anyVendorDeps`.
An entry with none of them — or with an empty `mayDependOn: []` — is rejected
with the error:

```
should have ref in 'mayDependOn'/'canUse' or at least one flag of
['anyProjectDeps', 'anyVendorDeps']
```

For leaf components that have no project-internal imports (stdlib and vendor
only), use `anyVendorDeps: true` instead of the empty list:

```yaml
# Wrong — v3 validator rejects this
cursor: { mayDependOn: [] }

# Correct — declares a permission that matches the actual import set
cursor: { anyVendorDeps: true }  # leaf: vendor-only deps (eris, std)
```

The comment on `anyVendorDeps` lines is load-bearing documentation: it tells
the next editor which vendor packages the component currently uses and why the
flag was chosen over a named list.

## Why this matters

The failure mode is silent mis-configuration: a plan document may write
`mayDependOn: []` for leaf components because that reads naturally ("depends on
nothing"), but the v3 validator treats it as an incomplete declaration and
halts the tool before any dependency check runs. The lint step exits non-zero
without naming a real violation — only the spec-validation error appears.

## Leaf vs. composition root vs. named list

| Situation | Use |
|---|---|
| Leaf: only stdlib / vendor imports | `anyVendorDeps: true` |
| Composition root (entry point that wires everything) | `anyProjectDeps: true` |
| Component with specific project-internal deps | `mayDependOn: [dep1, dep2, ...]` |
| Component with both project + vendor deps | `mayDependOn: [dep1, ...]` (vendor access is implicit) |

Do not combine `anyVendorDeps: true` with a non-empty `mayDependOn` list — the
spec allows it but the intent becomes ambiguous. Pick the most restrictive
permission set that matches the actual imports.

## Cross-reference

See [`.claude/rules/backend-layering.md` § "Add a new component"](../../../.claude/rules/backend-layering.md#operating-notes)
for the full checklist when adding a component, including when to use each
permission variant.
