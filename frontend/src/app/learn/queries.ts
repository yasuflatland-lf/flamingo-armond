import { graphql } from "@/generated";

export const LearnCardsByCardgroupQuery = graphql(`
  query LearnCardsByCardgroup($cardgroupId: ID!) {
    cardsByCardgroup(cardgroupId: $cardgroupId) {
      id
      front
      back
      due
      state
      cardgroupId
    }
  }
`);

export const HandleSwipeMutation = graphql(`
  mutation HandleSwipe($input: HandleSwipeInput!) {
    handleSwipe(input: $input) {
      nextCards {
        id
        front
        back
        due
        state
        cardgroupId
      }
      performanceMode
      metrics {
        successRate
        avgDifficulty
        retentionRate
        studyStreak
        lapseRate
        reviewCount
      }
    }
  }
`);

/**
 * Records the cardgroup the authenticated user most recently viewed on /learn.
 * Returns the updated user with the new lastViewedCardgroup so the Apollo cache
 * can write the fresh reference into the normalized User entity.
 */
export const SetLastViewedCardgroupMutation = graphql(`
  mutation SetLastViewedCardgroup($cardgroupId: ID!) {
    setLastViewedCardgroup(cardgroupId: $cardgroupId) {
      id
      lastViewedCardgroup {
        id
      }
    }
  }
`);
