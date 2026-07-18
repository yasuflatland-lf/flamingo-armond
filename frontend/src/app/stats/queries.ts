import { graphql } from "@/generated";

export const MyLearningStatsQuery = graphql(`
  query MyLearningStats {
    myLearningStats {
      ownsAnyDeck
      mastery { inProgress learned mature totalStudied }
      decks {
        cardgroup { id name }
        totalCards
        learnedCards
        matureCards
      }
      performanceWindows {
        days365 {
          retentionRate
          successRate
          lapseRate
          studyStreak
          reviewCount
          avgDifficulty
        }
        days30 {
          retentionRate
          successRate
          lapseRate
          studyStreak
          reviewCount
          avgDifficulty
        }
        days7 {
          retentionRate
          successRate
          lapseRate
          studyStreak
          reviewCount
          avgDifficulty
        }
      }
      strugglingCards {
        card { id front cardgroup { id name } }
        lapses
      }
    }
  }
`);

// Temporary rollout fallback for a frontend deployment that reaches the old
// backend before performanceWindows is available. Remove after the additive
// backend schema has been deployed everywhere and the old frontend is retired.
export const LegacyMyLearningStatsQuery = graphql(`
  query LegacyMyLearningStats {
    myLearningStats {
      ownsAnyDeck
      mastery { inProgress learned mature totalStudied }
      decks {
        cardgroup { id name }
        totalCards
        learnedCards
        matureCards
      }
      performance {
        retentionRate
        successRate
        lapseRate
        studyStreak
        reviewCount
        avgDifficulty
      }
      strugglingCards {
        card { id front cardgroup { id name } }
        lapses
      }
    }
  }
`);
