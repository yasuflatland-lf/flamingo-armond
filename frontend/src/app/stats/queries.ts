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
          knownReviewCount
          avgDifficulty
        }
        days30 {
          retentionRate
          successRate
          lapseRate
          studyStreak
          reviewCount
          knownReviewCount
          avgDifficulty
        }
        days7 {
          retentionRate
          successRate
          lapseRate
          studyStreak
          reviewCount
          knownReviewCount
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
