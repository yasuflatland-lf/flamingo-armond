package services_test

import (
	repository "backend/graph/db"
	"backend/graph/model"
	"backend/graph/services"
	repo "backend/pkg/repository"
	"backend/testutils"
	"context"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/labstack/gommon/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type CardTestSuite struct {
	suite.Suite
	db      *gorm.DB
	sv      services.Services
	cleanup func()
}

var migrationFilePath = "../../db/migrations"

func (suite *CardTestSuite) SetupSuite() {
	// Setup context
	ctx := context.Background()

	// Set up the test database
	pg, cleanup, err := testutils.SetupTestDB(ctx, "user", "password", "flamingo")
	if err != nil {
		log.Fatalf("Failed to setup test database: %+v", err)
	}
	suite.cleanup = func() {
		cleanup(migrationFilePath)
	}

	// Run migrations
	if err := pg.RunGooseMigrationsUp(migrationFilePath); err != nil {
		log.Fatalf("failed to run migrations: %+v", err)
	}

	// Setup Echo server
	suite.db = pg.GetDB()
	suite.sv = services.New(suite.db)
}

func (suite *CardTestSuite) TearDownSuite() {
	suite.cleanup()
}

func (suite *CardTestSuite) SetupSubTest() {
	t := suite.T()
	t.Helper()
	testutils.RunServersTest(t, suite.db, nil)
}

func (suite *CardTestSuite) TestCardService() {
	cardService := suite.sv.(services.CardService)
	cardGroupService := suite.sv.(services.CardGroupService)
	userService := suite.sv.(services.UserService)
	roleService := suite.sv.(services.RoleService)
	ctx := context.Background()
	t := suite.T()
	t.Helper()

	suite.Run("Normal_CreateCard", func() {
		// Arrange
		createdGroup, _, _ := testutils.CreateUserAndCardGroup(ctx, userService, cardGroupService, roleService)
		input := model.NewCard{
			Front:       "Test Front",
			Back:        "Test Back",
			ReviewDate:  time.Now().UTC(),
			CardgroupID: createdGroup.ID,
		}

		// Act
		createdCard, err := cardService.CreateCard(ctx, input)

		// Assert
		assert.NoError(suite.T(), err)
		assert.Equal(suite.T(), "Test Front", createdCard.Front)
		assert.Equal(suite.T(), "Test Back", createdCard.Back)
	})

	suite.Run("Error_InvalidCardGroupID", func() {
		// Arrange
		input := model.NewCard{
			Front:       "Test Front",
			Back:        "Test Back",
			ReviewDate:  time.Now().UTC(),
			CardgroupID: -1, // Invalid ID
		}

		// Act
		createdCard, err := cardService.CreateCard(ctx, input)

		// Assert
		assert.Error(suite.T(), err)
		assert.Nil(suite.T(), createdCard)
	})

	suite.Run("Normal_GetCardByID", func() {
		// Arrange
		cardGroup := model.NewCardGroup{Name: "Test Group"}
		createdGroup, _ := cardGroupService.CreateCardGroup(ctx, cardGroup)
		input := model.NewCard{
			Front:       "Test Front",
			Back:        "Test Back",
			ReviewDate:  time.Now().UTC(),
			CardgroupID: createdGroup.ID,
		}
		createdCard, _ := cardService.CreateCard(ctx, input)

		// Act
		fetchedCard, err := cardService.GetCardByID(ctx, createdCard.ID)

		// Assert
		assert.NoError(suite.T(), err)
		assert.Equal(suite.T(), createdCard.ID, fetchedCard.ID)
	})

	suite.Run("Error_CardNotFound", func() {
		// Act
		fetchedCard, err := cardService.GetCardByID(ctx, -1) // Invalid ID

		// Assert
		assert.Error(suite.T(), err)
		assert.Nil(suite.T(), fetchedCard)
	})

	suite.Run("Normal_UpdateCard", func() {
		// Arrange
		cardGroup := model.NewCardGroup{Name: "Test Group"}
		createdGroup, _ := cardGroupService.CreateCardGroup(ctx, cardGroup)
		input := model.NewCard{
			Front:       "Test Front",
			Back:        "Test Back",
			ReviewDate:  time.Now().UTC(),
			CardgroupID: createdGroup.ID,
		}
		createdCard, _ := cardService.CreateCard(ctx, input)

		updateInput := model.NewCard{
			Front:       "Updated Front",
			Back:        "Updated Back",
			ReviewDate:  time.Now().UTC(),
			CardgroupID: createdGroup.ID,
		}

		// Act
		updatedCard, err := cardService.UpdateCard(ctx, createdCard.ID, updateInput)

		// Assert
		assert.NoError(suite.T(), err)
		assert.Equal(suite.T(), "Updated Front", updatedCard.Front)
		assert.Equal(suite.T(), "Updated Back", updatedCard.Back)
	})

	suite.Run("Error_CardNotFound_Update", func() {
		// Arrange
		updateInput := model.NewCard{
			Front:      "Updated Front",
			Back:       "Updated Back",
			ReviewDate: time.Now().UTC(),
		}

		// Act
		updatedCard, err := cardService.UpdateCard(ctx, -1, updateInput) // Invalid ID

		// Assert
		assert.Error(suite.T(), err)
		assert.Nil(suite.T(), updatedCard)
	})

	suite.Run("Normal_DeleteCard", func() {
		// Arrange
		cardGroup := model.NewCardGroup{Name: "Test Group"}
		createdGroup, _ := cardGroupService.CreateCardGroup(ctx, cardGroup)
		input := model.NewCard{
			Front:       "Test Front",
			Back:        "Test Back",
			ReviewDate:  time.Now().UTC(),
			CardgroupID: createdGroup.ID,
		}
		createdCard, _ := cardService.CreateCard(ctx, input)

		// Act
		deleted, err := cardService.DeleteCard(ctx, createdCard.ID)

		// Assert
		assert.NoError(suite.T(), err)
		assert.True(suite.T(), *deleted)
	})

	suite.Run("Error_CardNotFound_Delete", func() {
		// Act
		deleted, err := cardService.DeleteCard(ctx, -1) // Invalid ID

		// Assert
		assert.Error(suite.T(), err)
		assert.False(suite.T(), *deleted)
	})

	suite.Run("Normal_FetchAllCardsByCardGroup", func() {
		// Arrange
		createdGroup, _, _ := testutils.CreateUserAndCardGroup(ctx, userService, cardGroupService, roleService)

		// Create 200 dummy cards
		for i := 0; i < 200; i++ {
			input := model.NewCard{
				Front:       "Front " + strconv.Itoa(i),
				Back:        "Back " + strconv.Itoa(i),
				ReviewDate:  time.Now().UTC(),
				CardgroupID: createdGroup.ID,
			}
			_, err := cardService.CreateCard(ctx, input)
			assert.NoError(t, err)
		}

		// Act
		var first = 8
		allCards, err := cardService.FetchAllCardsByCardGroup(ctx, createdGroup.ID, &first)

		// Assert
		assert.NoError(t, err)
		assert.Len(t, allCards, 200) // Ensure all 200 cards are retrieved
	})

	suite.Run("Normal_AddNewCards_Matching5Of8", func() {
		// Arrange
		cardGroup := model.NewCardGroup{Name: "Test Group"}
		createdGroup, _ := cardGroupService.CreateCardGroup(ctx, cardGroup)

		// Create existing 5 cards
		existingCards := []model.Card{}
		for i := 0; i < 5; i++ {
			input := model.NewCard{
				Front:       "Front " + strconv.Itoa(i),
				Back:        "Back " + strconv.Itoa(i),
				ReviewDate:  time.Now().UTC(),
				CardgroupID: createdGroup.ID,
			}
			card, err := cardService.CreateCard(ctx, input)
			assert.NoError(t, err)
			existingCards = append(existingCards, *card)
		}

		// Create 8 target cards (5 matching existing, 3 new)
		targetCards := []model.Card{}
		for i := 0; i < 8; i++ {
			targetCard := model.Card{
				Front:       "Front " + strconv.Itoa(i),
				Back:        "Back " + strconv.Itoa(i),
				ReviewDate:  time.Now().UTC(),
				CardGroupID: createdGroup.ID,
			}
			targetCards = append(targetCards, targetCard)
		}

		// Act
		_, err := cardService.AddNewCards(ctx, targetCards, createdGroup.ID)

		// Assert
		assert.NoError(t, err)
		allCards, err := cardService.FetchAllCardsByCardGroup(ctx, createdGroup.ID, nil)
		assert.NoError(t, err)
		assert.Len(t, allCards, 8) // Ensure all 8 cards are present, 3 added
	})

	suite.Run("Normal_AddNewCards_Matching3Of7", func() {
		// Arrange
		cardGroup := model.NewCardGroup{Name: "Test Group"}
		createdGroup, _ := cardGroupService.CreateCardGroup(ctx, cardGroup)

		// Create existing 5 cards
		existingCards := []model.Card{}
		for i := 0; i < 5; i++ {
			input := model.NewCard{
				Front:       "Front " + strconv.Itoa(i),
				Back:        "Back " + strconv.Itoa(i),
				ReviewDate:  time.Now().UTC(),
				CardgroupID: createdGroup.ID,
			}
			card, err := cardService.CreateCard(ctx, input)
			assert.NoError(t, err)
			existingCards = append(existingCards, *card)
		}

		// Create 7 target cards (3 matching existing, 4 new)
		targetCards := []model.Card{}
		for i := 2; i < 9; i++ {
			targetCard := model.Card{
				Front:       "Front " + strconv.Itoa(i),
				Back:        "Back " + strconv.Itoa(i),
				ReviewDate:  time.Now().UTC(),
				CardGroupID: createdGroup.ID,
			}
			targetCards = append(targetCards, targetCard)
		}

		// Act
		_, err := cardService.AddNewCards(ctx, targetCards, createdGroup.ID)

		// Assert
		assert.NoError(t, err)
		allCards, err := cardService.FetchAllCardsByCardGroup(ctx, createdGroup.ID, nil)
		assert.NoError(t, err)
		assert.Len(t, allCards, 9) // 4 new cards added to the 5 existing ones
	})

	suite.Run("Normal_AddNewCards_NoTargetCards", func() {
		// Arrange
		cardGroup := model.NewCardGroup{Name: "Test Group"}
		createdGroup, _ := cardGroupService.CreateCardGroup(ctx, cardGroup)

		// Create existing 5 cards
		existingCards := []model.Card{}
		for i := 0; i < 5; i++ {
			input := model.NewCard{
				Front:       "Front " + strconv.Itoa(i),
				Back:        "Back " + strconv.Itoa(i),
				ReviewDate:  time.Now().UTC(),
				CardgroupID: createdGroup.ID,
			}
			card, err := cardService.CreateCard(ctx, input)
			assert.NoError(t, err)
			existingCards = append(existingCards, *card)
		}

		// No target cards
		targetCards := []model.Card{}

		// Act
		_, err := cardService.AddNewCards(ctx, targetCards, createdGroup.ID)

		// Assert
		assert.NoError(t, err)
		allCards, err := cardService.FetchAllCardsByCardGroup(ctx, createdGroup.ID, nil)
		assert.NoError(t, err)
		assert.Len(t, allCards, 5) // No new cards added
	})

	suite.Run("Normal_AddNewCards_EmptyExistingWithTargetCards", func() {
		// Arrange
		cardGroup := model.NewCardGroup{Name: "Test Group"}
		createdGroup, _ := cardGroupService.CreateCardGroup(ctx, cardGroup)

		// Create 5 target cards
		targetCards := []model.Card{}
		for i := 0; i < 5; i++ {
			targetCard := model.Card{
				Front:       "Front " + strconv.Itoa(i),
				Back:        "Back " + strconv.Itoa(i),
				ReviewDate:  time.Now().UTC(),
				CardGroupID: createdGroup.ID,
			}
			targetCards = append(targetCards, targetCard)
		}

		// Act
		_, err := cardService.AddNewCards(ctx, targetCards, createdGroup.ID)

		// Assert
		assert.NoError(t, err)
		allCards, err := cardService.FetchAllCardsByCardGroup(ctx, createdGroup.ID, nil)
		assert.NoError(t, err)
		assert.Len(t, allCards, 5) // All 5 target cards added
	})

	suite.Run("Normal_AddNewCards_BothEmpty", func() {
		// Arrange
		cardGroup := model.NewCardGroup{Name: "Test Group"}
		createdGroup, _ := cardGroupService.CreateCardGroup(ctx, cardGroup)

		// No target cards
		targetCards := []model.Card{}

		// Act
		_, err := cardService.AddNewCards(ctx, targetCards, createdGroup.ID)

		// Assert
		assert.NoError(t, err)
		allCards, err := cardService.FetchAllCardsByCardGroup(ctx, createdGroup.ID, nil)
		assert.NoError(t, err)
		assert.Len(t, allCards, 0) // No cards present
	})

	suite.Run("Normal_AddNewCards_UpdateOnBackDifference", func() {
		// Arrange
		cardGroup := model.NewCardGroup{Name: "Test Group"}
		createdGroup, _ := cardGroupService.CreateCardGroup(ctx, cardGroup)

		// Create an existing card with specific Front and Back
		intervalDays := 1 // Set an appropriate default value
		input := model.NewCard{
			Front:        "Test Front",
			Back:         "Test Back",
			ReviewDate:   time.Now().UTC(),
			IntervalDays: &intervalDays, // Set IntervalDays here
			CardgroupID:  createdGroup.ID,
		}
		createdCard, err := cardService.CreateCard(ctx, input)
		assert.NoError(t, err)

		// Create a target card with the same Front but slightly different Back
		targetCard := model.Card{
			Front:        "Test Front",
			Back:         "Test BackX", // 1 character difference
			ReviewDate:   time.Now().UTC(),
			IntervalDays: intervalDays, // Similarly set IntervalDays
			CardGroupID:  createdGroup.ID,
		}

		// Act
		_, err = cardService.AddNewCards(ctx, []model.Card{targetCard}, createdGroup.ID)

		// Assert
		assert.NoError(t, err)
		updatedCard, err := cardService.GetCardByID(ctx, createdCard.ID)
		assert.NoError(t, err)
		assert.Equal(t, "Test BackX", updatedCard.Back) // Ensure the Back was updated
		assert.Equal(t, createdCard.ID, updatedCard.ID) // Ensure the same card ID is retained
	})

	suite.Run("Normal_GetCardsByUserAndCardGroup", func() {
		// Create a user and a card group
		createdGroup, _, _ := testutils.CreateUserAndCardGroup(ctx, userService, cardGroupService, roleService)

		// Create some cards
		for i := 0; i < 5; i++ {
			input := model.NewCard{
				Front:       "Front " + strconv.Itoa(i),
				Back:        "Back " + strconv.Itoa(i),
				ReviewDate:  time.Now().UTC(),
				CardgroupID: createdGroup.ID,
			}
			_, err := cardService.CreateCard(ctx, input)
			assert.NoError(suite.T(), err)
		}

		// Act
		cards, err := cardService.GetCardsByUserAndCardGroup(ctx, createdGroup.ID, repo.DESC, 3)

		// Assert
		assert.NoError(suite.T(), err)
		assert.Len(suite.T(), cards, 3)
		for i := 0; i < 3; i++ {
			assert.Equal(suite.T(), "Front "+strconv.Itoa(4-i), cards[i].Front)
		}
	})

	suite.Run("Error_GetCardsByUserAndCardGroup_NoCards", func() {
		// Arrange
		createdGroup, _, _ := testutils.CreateUserAndCardGroup(ctx, userService, cardGroupService, roleService)

		// Act
		cards, err := cardService.GetCardsByUserAndCardGroup(ctx, createdGroup.ID, "desc", 3)

		// Assert
		assert.NoError(suite.T(), err)
		assert.Empty(suite.T(), cards)
	})

	suite.Run("Normal_GetRandomCardsFromRecentUpdates", func() {
		// Arrange
		createdGroup, _, _ := testutils.CreateUserAndCardGroup(ctx, userService, cardGroupService, roleService)

		// Create 50 dummy cards
		for i := 0; i < 50; i++ {
			input := model.NewCard{
				Front:       "Front " + strconv.Itoa(i),
				Back:        "Back " + strconv.Itoa(i),
				ReviewDate:  time.Now().UTC(),
				CardgroupID: createdGroup.ID,
			}
			_, err := cardService.CreateCard(ctx, input)
			assert.NoError(suite.T(), err)
		}

		// Act
		limit := 10
		randomCards1, err := cardService.GetRandomCardsFromRecentUpdates(ctx, createdGroup.ID, limit, repo.DESC, repo.ASC)
		assert.NoError(suite.T(), err)
		assert.Len(suite.T(), randomCards1, limit) // Ensure that 10 cards are returned

		randomCards2, err := cardService.GetRandomCardsFromRecentUpdates(ctx, createdGroup.ID, limit, repo.DESC, repo.ASC)
		assert.NoError(suite.T(), err)
		assert.Len(suite.T(), randomCards2, limit) // Ensure that 10 cards are returned

		// Assert
		sameOrder := true
		for i := range randomCards1 {
			if randomCards1[i].ID != randomCards2[i].ID {
				sameOrder = false
				break
			}
		}

		// Ensure the order is different (i.e., shuffled)
		assert.False(suite.T(), sameOrder, "The order of cards should be different between the two calls")
	})

	suite.Run("Normal_GetRandomCardsFromRecentUpdates_LessThanLimit", func() {
		// Arrange
		createdGroup, _, _ := testutils.CreateUserAndCardGroup(ctx, userService, cardGroupService, roleService)

		// Create 5 dummy cards
		for i := 0; i < 5; i++ {
			input := model.NewCard{
				Front:       "Front " + strconv.Itoa(i),
				Back:        "Back " + strconv.Itoa(i),
				ReviewDate:  time.Now().UTC(),
				CardgroupID: createdGroup.ID,
			}
			_, err := cardService.CreateCard(ctx, input)
			assert.NoError(suite.T(), err)
		}

		// Act
		limit := 10
		randomCards, err := cardService.GetRandomCardsFromRecentUpdates(ctx, createdGroup.ID, limit, repo.DESC, repo.ASC)

		// Assert
		assert.NoError(suite.T(), err)
		assert.Len(suite.T(), randomCards, 5) // Ensure that only the available 5 cards are returned
	})

	suite.Run("Normal_GetRandomCardsFromRecentUpdates_InvalidSortOrders", func() {
		// Arrange
		createdGroup, _, _ := testutils.CreateUserAndCardGroup(ctx, userService, cardGroupService, roleService)

		// Create 20 dummy cards
		for i := 0; i < 20; i++ {
			input := model.NewCard{
				Front:       "Front " + strconv.Itoa(i),
				Back:        "Back " + strconv.Itoa(i),
				ReviewDate:  time.Now().UTC(),
				CardgroupID: createdGroup.ID,
			}
			_, err := cardService.CreateCard(ctx, input)
			assert.NoError(suite.T(), err)
		}

		// Act
		limit := 10
		randomCards, err := cardService.GetRandomCardsFromRecentUpdates(ctx, createdGroup.ID, limit, "invalid_order", "invalid_order")

		// Assert
		assert.NoError(suite.T(), err)
		assert.Len(suite.T(), randomCards, limit) // Ensure that 10 cards are returned
	})

	suite.Run("Normal_GetCardsForReview", func() {
		// Arrange
		createdGroup, _, _ := testutils.CreateUserAndCardGroup(ctx, userService, cardGroupService, roleService)

		now := time.Now().UTC()
		r := rand.New(rand.NewSource(time.Now().UnixNano())) // New random number generator

		// Create 100 cards with random updated times and interval days
		for i := 0; i < 100; i++ {
			intervalDays := r.Intn(50) + 1 // Random IntervalDays between 1 and 50
			input := model.NewCard{
				Front:        "Front " + strconv.Itoa(i),
				Back:         "Back " + strconv.Itoa(i),
				ReviewDate:   now.AddDate(0, 0, -i), // Spread the review dates
				IntervalDays: &intervalDays,
				CardgroupID:  createdGroup.ID,
			}
			card, err := cardService.CreateCard(ctx, input)
			assert.NoError(t, err)

			// Ensure the Front field is always populated
			assert.NotEmpty(t, card.Front, "Card Front field should not be empty")

			// Adjust the updated date to simulate a delay
			card.Updated = now.AddDate(0, 0, -i*2)
			err = suite.db.Save(&repository.Card{
				ID:           card.ID,
				Front:        card.Front, // Ensure Front is populated
				Back:         card.Back,  // Ensure Back is populated
				ReviewDate:   card.ReviewDate,
				CardGroupID:  createdGroup.ID,
				Updated:      card.Updated,
				IntervalDays: *input.IntervalDays,
			}).Error
			assert.NoError(t, err)
		}

		// Act
		cards, err := cardService.GetCardsByDefaultLogic(ctx, createdGroup.ID, 10)

		// Assert
		assert.NoError(t, err)
		assert.Len(t, cards, 10) // Ensure that only 10 cards are returned
		// Ensure that cards are ordered by their next review date (Updated + IntervalDays)
		for i := 1; i < len(cards); i++ {
			previousReviewTime := cards[i-1].Updated.Add(time.Hour * 24 * time.Duration(cards[i-1].IntervalDays))
			currentReviewTime := cards[i].Updated.Add(time.Hour * 24 * time.Duration(cards[i].IntervalDays))
			assert.True(t, previousReviewTime.Before(currentReviewTime) || previousReviewTime.Equal(currentReviewTime))
		}
	})

	suite.Run("Normal_ShuffleCards", func() {
		// Arrange
		createdGroup, _, _ := testutils.CreateUserAndCardGroup(ctx, userService, cardGroupService, roleService)

		// Create 10 dummy cards and associate them with the created card group
		limit := 10
		cards := []repository.Card{}
		for i := 0; i < limit; i++ {
			card := repository.Card{
				Front:        "Front " + strconv.Itoa(i),
				Back:         "Back " + strconv.Itoa(i),
				ReviewDate:   time.Now().UTC(),
				IntervalDays: 1,
				Created:      time.Now().UTC(),
				Updated:      time.Now().UTC(),
				CardGroupID:  createdGroup.ID,
				CardGroup: repository.Cardgroup{
					ID: createdGroup.ID,
				},
			}
			cards = append(cards, card)
		}

		// Act
		shuffledCards := cardService.ShuffleCards(cards, limit)

		// Assert
		assert.Len(suite.T(), shuffledCards, len(cards))
		assert.NotEqual(suite.T(), cards, shuffledCards, "Shuffled cards should not be in the same order as the original")

		// Check if all original cards are still present after shuffle
		cardMap := make(map[int64]*model.Card)
		for _, card := range shuffledCards {
			cardMap[card.ID] = card
		}
		for _, card := range cards {
			_, exists := cardMap[card.ID]
			assert.True(suite.T(), exists, "All original cards should be present after shuffle")
		}
	})

	suite.Run("Normal_ShuffleCards_Randomness", func() {
		// Arrange
		createdGroup, _, _ := testutils.CreateUserAndCardGroup(ctx, userService, cardGroupService, roleService)

		// Create 10 dummy cards and associate them with the created card group
		limit := 10
		cards := []repository.Card{}
		for i := 0; i < limit; i++ {
			card := repository.Card{
				Front:        "Front " + strconv.Itoa(i),
				Back:         "Back " + strconv.Itoa(i),
				ReviewDate:   time.Now().UTC(),
				IntervalDays: 1,
				Created:      time.Now().UTC(),
				Updated:      time.Now().UTC(),
				CardGroupID:  createdGroup.ID,
				CardGroup: repository.Cardgroup{
					ID: createdGroup.ID,
				},
			}
			cards = append(cards, card)
		}

		// Shuffle twice
		shuffledCards1 := cardService.ShuffleCards(cards, limit)
		shuffledCards2 := cardService.ShuffleCards(cards, limit)

		// Assert that the order is different in at least one shuffle
		assert.NotEqual(suite.T(), shuffledCards1, shuffledCards2, "Different shuffles should result in different orders")
	})

	suite.Run("Normal_GetRecentCards", func() {
		// Arrange
		cardService := suite.sv.(services.CardService)
		userService := suite.sv.(services.UserService)
		cardGroupService := suite.sv.(services.CardGroupService)
		roleService := suite.sv.(services.RoleService)
		ctx := context.Background()

		// Create a user and card group using testutils
		createdGroup, _, _ := testutils.CreateUserAndCardGroup(ctx, userService, cardGroupService, roleService)

		// Add some cards with different creation times
		for i := 0; i < 10; i++ {
			input := model.NewCard{
				Front:       "Recent Front " + strconv.Itoa(i),
				Back:        "Recent Back " + strconv.Itoa(i),
				ReviewDate:  time.Now().UTC().Add(-time.Duration(i) * time.Hour),
				CardgroupID: createdGroup.ID,
			}
			_, err := cardService.CreateCard(ctx, input)
			assert.NoError(suite.T(), err)
		}

		// Define the date from which to fetch recent cards
		fromDate := time.Now().UTC().Add(-5 * time.Hour) // fetch cards created in the last 5 hours
		limit := 5

		// Act
		recentCards, err := cardService.GetRandomRecentCards(ctx, fromDate, limit,
			repo.DESC)

		// Assert
		assert.NoError(suite.T(), err)
		assert.Len(suite.T(), recentCards, limit)
		for _, card := range recentCards {
			assert.True(suite.T(), card.Created.After(fromDate), "Card should be created after the fromDate")
		}
	})

	suite.Run("Normal_GetRecentCards_LimitExceeds", func() {
		// Arrange
		cardService := suite.sv.(services.CardService)
		userService := suite.sv.(services.UserService)
		cardGroupService := suite.sv.(services.CardGroupService)
		roleService := suite.sv.(services.RoleService)
		ctx := context.Background()

		// Reuse the same method to create a user and card group
		createdGroup, _, _ := testutils.CreateUserAndCardGroup(ctx, userService, cardGroupService, roleService)

		// Add some cards with different creation times
		for i := 0; i < 10; i++ {
			input := model.NewCard{
				Front:       "Recent Front " + strconv.Itoa(i),
				Back:        "Recent Back " + strconv.Itoa(i),
				ReviewDate:  time.Now().UTC().Add(-time.Duration(i) * time.Hour),
				CardgroupID: createdGroup.ID,
			}
			_, err := cardService.CreateCard(ctx, input)
			assert.NoError(suite.T(), err)
		}

		// Define the date from which to fetch recent cards
		fromDate := time.Now().UTC().Add(-24 * time.Hour)
		limit := 15 // Exceeds the number of available cards

		// Act
		recentCards, err := cardService.GetRandomRecentCards(ctx, fromDate, limit,
			repo.DESC)

		// Assert
		assert.NoError(suite.T(), err)
		assert.True(suite.T(), len(recentCards) <= limit, "Number of cards should not exceed the limit")
		for _, card := range recentCards {
			assert.True(suite.T(), card.Created.After(fromDate), "Card should be created after the fromDate")
		}
	})

}

func TestCardTestSuite(t *testing.T) {
	suite.Run(t, new(CardTestSuite))
}
