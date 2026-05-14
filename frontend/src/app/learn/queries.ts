import { graphql } from "@/generated";

/** Number of cards requested per prefetch and SSR load. */
export const LEARN_PAGE_LIMIT = 20;

export const LearnNextDueCardsQuery = graphql(`
  query LearnNextDueCards($cardgroupId: ID!, $limit: Int = 20) {
    learnNextDueCards(cardgroupId: $cardgroupId, limit: $limit) {
      id
      front
      back
      userCardState {
        due
        state
      }
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
        userCardState {
          due
          state
        }
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
