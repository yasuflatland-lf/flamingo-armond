// ESLint flat config scoped to the shared GraphQL schema (`schema/*.graphql`).
//
// The frontend uses Biome for TS/JS; ESLint exists only to lint the schema
// for the conventions documented in issue #162 (camelCase fields, PascalCase
// types, doc-blocks on root fields and object types, no typename prefix, and
// `@deprecated` reason-presence).
//
// Run: `pnpm lint:schema` (or `pnpm exec eslint schema/**/*.graphql`).
import graphqlPlugin from "@graphql-eslint/eslint-plugin";

export default [
  {
    files: ["schema/**/*.graphql"],
    languageOptions: {
      parser: graphqlPlugin.parser,
      parserOptions: {
        graphQLConfig: {
          schema: "schema/schema.graphql",
        },
      },
    },
    plugins: {
      "@graphql-eslint": graphqlPlugin,
    },
    rules: {
      "@graphql-eslint/naming-convention": [
        "error",
        {
          FieldDefinition: "camelCase",
          ObjectTypeDefinition: "PascalCase",
          InterfaceTypeDefinition: "PascalCase",
          InputObjectTypeDefinition: "PascalCase",
          UnionTypeDefinition: "PascalCase",
          EnumTypeDefinition: "PascalCase",
          EnumValueDefinition: "UPPER_CASE",
          ScalarTypeDefinition: "PascalCase",
        },
      ],
      "@graphql-eslint/require-description": [
        "error",
        { types: true, rootField: true },
      ],
      "@graphql-eslint/require-deprecation-reason": "error",
      "@graphql-eslint/no-typename-prefix": "error",
    },
  },
];
