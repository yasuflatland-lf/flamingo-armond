import { graphql } from "@/generated";

export const HeaderMeQuery = graphql(`
  query HeaderMe {
    me {
      roles {
        name
      }
    }
  }
`);
