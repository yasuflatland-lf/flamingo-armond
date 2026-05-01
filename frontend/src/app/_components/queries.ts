import { graphql } from "@/generated";

// Header queries

export const HeaderMeQuery = graphql(`
  query HeaderMe {
    me {
      roles {
        name
      }
    }
  }
`);
