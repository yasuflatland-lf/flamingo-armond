import { graphql } from "@/generated";

/**
 * Bootstrap query for the HomePage RSC.
 *
 * Used to decide where to redirect a signed-in caller:
 *   - me.lastViewedCardgroup != null            → /learn/{id}
 *   - myCardgroupsConnection.totalCount > 0     → /cardgroups
 *   - otherwise                                  → /cardgroups/new?welcome=1
 *
 * Auth-sensitive (requires Authorization), so callers MUST pass `revalidate: 0`
 * to gqlFetch.
 */
export const MeWithLastViewedQuery = graphql(`
  query MeWithLastViewed {
    me {
      id
      displayName
      lastViewedCardgroup {
        id
      }
    }
    myCardgroupsConnection(first: 1) {
      totalCount
    }
  }
`);
