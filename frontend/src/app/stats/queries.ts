import { graphql } from "@/generated";

export const MyLearningStatsQuery = graphql(`
  query MyLearningStats {
    myLearningStats {
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
        stability
      }
    }
  }
`);
