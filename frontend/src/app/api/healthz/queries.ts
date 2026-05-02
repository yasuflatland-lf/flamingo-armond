import { graphql } from "@/generated";

export const HealthzQuery = graphql(`
  query Healthz {
    health
  }
`);
