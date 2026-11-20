import { graphql } from "@/generated";

export const VerifySessionQuery = graphql(`
  query VerifySession {
    me {
      id
    }
  }
`);
