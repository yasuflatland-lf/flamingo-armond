---
name: codegen-sync
description: Regenerate GraphQL artifacts (gqlgen + graphql-codegen + Next typegen) in the correct order and smoke-build, after editing schema/schema.graphql. Handles the two-pass schema-field-drop failure mode.
disable-model-invocation: true
---

Run after any change to `schema/schema.graphql`.

1. Regenerate everything: `make codegen` (runs gqlgen for backend and
   graphql-codegen for frontend). If it fails on a *removed* schema field, that
   is the documented two-pass failure: regen cannot succeed until the production
   Go/TS code that referenced the field is updated. Fix the referencing code,
   then re-run. See docs/backend/library-gotchas/codegen-two-pass-schema-field-drop.md.
2. Smoke-build backend: `cd backend && go build ./...`.
3. Typecheck frontend: `cd frontend && pnpm typecheck`.
4. Report which generated files changed (`git status --short backend/graph frontend/src/generated`)
   and whether builds passed. Do NOT commit — leave staging to the user.
