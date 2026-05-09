# Why `eris` over alternatives

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

- **`fmt.Errorf("%w")`**: no stack trace, can only carry a string context.
- **`pkg/errors`**: last tagged release v0.9.1 in January 2020 with no upstream activity since; lacks the structured JSON chain serialization that this codebase relies on for the `error_chain` log attribute.
- **`cockroachdb/errors`**: heavier, drags in many transitive deps; revisit only when multi-service error portability or first-class Sentry SDK integration becomes a hard requirement.
- **`joomcode/errorx`**: typed-error focus, less aligned with our wrap-and-log need.
