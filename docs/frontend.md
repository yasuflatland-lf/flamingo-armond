# Frontend / schema notes

## Current state

`frontend/` and `schema/` are **scaffolded but empty**:

- `frontend/package.json` has `name`, `private`, and `version` set but **no dependencies or scripts**. Do not propose `pnpm` / `npm` / `yarn` / `bun` / codegen commands without first reading `package.json`. Guessing will mislead.
- `frontend/codegen.ts` is an empty file.
- `frontend/src/` contains only `.gitkeep`.
- `schema/` contains only `.gitkeep`.

## Planned architecture (pre-implementation)

The architectural pivot this repo is being built around is **shared `schema/` → codegen on both sides**:

- `backend/gqlgen.yml` will consume `schema/*.graphql` and generate into `backend/graph/{generated,model,resolver}/`.
- `frontend/codegen.ts` will consume the same `schema/*.graphql` and generate types under `frontend/src/`.

Until `schema/` contains `.graphql` files, both `gqlgen` and `codegen` will either no-op or error. **Do not invoke them against an empty schema.**
