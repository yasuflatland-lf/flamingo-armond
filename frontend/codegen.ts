import type { CodegenConfig } from "@graphql-codegen/cli";

// The `Time` scalar is serialized as an ISO-8601 string over JSON. Pin it to
// `string` so consumers don't get the v6 default of `unknown`.
const scalars = { Time: "string" } as const;

const config: CodegenConfig = {
  schema: "../schema/*.graphql",
  documents: ["src/**/*.{ts,tsx}", "!src/generated/**"],
  generates: {
    "src/generated/": {
      preset: "client",
      config: {
        useTypeImports: true,
        scalars,
        // client-preset v6 stopped injecting `__typename` unless selected.
        // Apollo Client auto-requests it on every non-root selection, so add
        // it back to keep generated result types aligned with the cache.
        nonOptionalTypename: true,
        skipTypeNameForRoot: true,
      },
    },
    // client-preset v6 only emits operation/input/enum types. Base schema
    // object types (Card, User, Role, ...) are generated here for test
    // fixtures that build mock payloads from the full schema shape.
    "src/generated/base-types.ts": {
      plugins: ["typescript"],
      config: {
        useTypeImports: true,
        scalars,
        // Match the operation types: `__typename` is required and nullable
        // fields are `T | null` (not optional `T | null | undefined`), so
        // base-type fixtures stay assignable to operation result types.
        nonOptionalTypename: true,
        avoidOptionals: true,
      },
    },
  },
  ignoreNoDocuments: true,
};

export default config;
