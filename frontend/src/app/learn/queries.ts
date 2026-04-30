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
