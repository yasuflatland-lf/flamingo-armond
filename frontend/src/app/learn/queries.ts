import { graphql } from "@/generated";

/** Number of cards requested per prefetch and SSR load. */
export const LEARN_PAGE_LIMIT = 20;

export const LearnNextDueCardsQuery = graphql(`
  query LearnNextDueCards($cardgroupId: ID!, $limit: Int = 20) {
    learnNextDueCards(cardgroupId: $cardgroupId, limit: $limit) {
      id
      front
      back
      cefrLevel
      userCardState {
        due
        state
      }
      cardgroupId
    }
    me {
      learnDisplayMode
    }
  }
`);

/**
 * Read-only pool of cards already reviewed today, used by practice mode.
 *
 * Practice mode NEVER writes — swiping a practice card fires no mutation and has
 * no FSRS impact. The selection set mirrors `SwipeCardData` (the shape consumed
 * by `SwipeCardStack`), including `userCardState` because that component reads
 * it; the practice client itself ignores the FSRS fields.
 */
export const PracticeTodaysCardsQuery = graphql(`
  query PracticeTodaysCards($cardgroupId: ID!, $limit: Int = 100) {
    practiceTodaysCards(cardgroupId: $cardgroupId, limit: $limit) {
      id
      front
      back
      cefrLevel
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
      __typename
      ... on HandleSwipeSuccess {
        response {
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
      ... on InputValidationError {
        field
        message
      }
    }
  }
`);

export const SetLastViewedCardgroupMutation = graphql(`
  mutation SetLastViewedCardgroup($cardgroupId: ID!) {
    setLastViewedCardgroup(cardgroupId: $cardgroupId) {
      __typename
      ... on SetLastViewedCardgroupSuccess {
        user {
          id
          lastViewedCardgroup {
            id
          }
        }
      }
      ... on InputValidationError {
        field
        message
      }
    }
  }
`);

export const UpdateLearnDisplayModeMutation = graphql(`
  mutation UpdateLearnDisplayMode($mode: LearnDisplayMode!) {
    updateLearnDisplayMode(mode: $mode) {
      id
      learnDisplayMode
    }
  }
`);
