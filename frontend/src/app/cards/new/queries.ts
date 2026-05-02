import { graphql } from "@/generated";

/**
 * Bootstrap query for the /cards/new route.
 * Returns the user's last-viewed cardgroup (used as a fallback when no
 * ?cardgroup= search param is present) together with the full list of owned
 * cardgroups (used for ownership validation and to seed the picker).
 */
export const CardsNewBootstrapQuery = graphql(`
  query CardsNewBootstrap {
    me {
      id
      lastViewedCardgroup {
        id
      }
    }
    myCardgroups {
      id
      name
    }
  }
`);
