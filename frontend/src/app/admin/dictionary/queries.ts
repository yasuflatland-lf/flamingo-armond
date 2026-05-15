import { graphql } from "@/generated";

// Re-export the Connection query so the cardgroup selector shares the same
// document and Apollo cache key as the main cardgroups page.
export { MyCardgroupsConnectionQuery as AdminDictionaryCardgroupsConnectionQuery } from "@/app/cardgroups/queries";

export const ValidateDictionaryQuery = graphql(`
  query ValidateDictionary($input: ValidateDictionaryInput!) {
    validateDictionary(input: $input) {
      valid
      parsedWords {
        front
        back
        line
      }
      errors {
        line
        message
      }
    }
  }
`);

export const UpsertDictionaryMutation = graphql(`
  mutation UpsertDictionary($input: UpsertDictionaryInput!) {
    upsertDictionary(input: $input) {
      inserted
      updated
      errors {
        line
        message
      }
    }
  }
`);
