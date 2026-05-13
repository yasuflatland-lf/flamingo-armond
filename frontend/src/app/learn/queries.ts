import { graphql } from "@/generated";

export const LearnNextDueCardsQuery = graphql(`
  query LearnNextDueCards($cardgroupId: ID!, $limit: Int = 20) {
    learnNextDueCards(cardgroupId: $cardgroupId, limit: $limit) {
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
