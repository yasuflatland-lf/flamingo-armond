package services

import (
	repository "backend/graph/db"
	"backend/graph/model"
	"backend/pkg/logger"
	repo "backend/pkg/repository"
	"backend/pkg/utils"
	"context"
	"errors"
	"fmt"
	"github.com/m-mizutani/goerr"
	"math/rand"
	"strings"
	"time"

	"gorm.io/gorm"
)

type cardService struct {
	db           *gorm.DB
	defaultLimit int
}

type CardService interface {
	GetCardByID(ctx context.Context, id int64) (*model.Card, error)
	CreateCard(ctx context.Context, input model.NewCard) (*model.Card, error)
	UpdateCard(ctx context.Context, id int64, input model.NewCard) (*model.Card, error)
	DeleteCard(ctx context.Context, id int64) (*bool, error)
	PaginatedCardsByCardGroup(ctx context.Context, cardGroupID int64, first *int, after *int64, last *int, before *int64) (*model.CardConnection, error)
	GetCardsByIDs(ctx context.Context, ids []int64) ([]*model.Card, error)
	FetchAllCardsByCardGroup(ctx context.Context, cardGroupID int64, first *int) ([]*model.Card, error)
	AddNewCards(ctx context.Context, targetCards []model.Card, cardGroupID int64) ([]*model.Card, error)
	GetCardsByUserAndCardGroup(ctx context.Context, cardGroupID int64, order string, limit int) ([]repository.Card, error)
	GetRandomCardsFromRecentUpdates(ctx context.Context, cardGroupID int64, limit int, updatedSortOrder string, intervalDaysSortOrder string) ([]model.Card, error)
}

func NewCardService(db *gorm.DB, defaultLimit int) CardService {
	return &cardService{db: db, defaultLimit: defaultLimit}
}

func ConvertToGormCard(input model.NewCard) *repository.Card {
	return &repository.Card{
		Front:      input.Front,
		Back:       input.Back,
		ReviewDate: input.ReviewDate,
		IntervalDays: func() int {
			if input.IntervalDays != nil {
				return *input.IntervalDays
			}
			return 1
		}(),
		CardGroupID: input.CardgroupID,
		Created:     input.Created,
		Updated:     input.Updated,
	}
}

func ConvertToCard(card repository.Card) *model.Card {
	return &model.Card{
		ID:           card.ID,
		Front:        card.Front,
		Back:         card.Back,
		ReviewDate:   card.ReviewDate,
		IntervalDays: card.IntervalDays,
		CardGroupID:  card.CardGroupID,
		Created:      card.Created,
		Updated:      card.Updated,
	}
}

func ConvertToCards(cards []repository.Card) []model.Card {
	var result []model.Card
	for _, card := range cards {
		convertedCard := ConvertToCard(card)
		result = append(result, *convertedCard)
	}
	return result
}

func convertCardConnection(cards []repository.Card, hasPrevPage, hasNextPage bool) *model.CardConnection {
	var result model.CardConnection

	for _, dbc := range cards {
		card := ConvertToCard(dbc)

		// Use the ID directly as it is already of type int64
		result.Edges = append(result.Edges, &model.CardEdge{Cursor: card.ID, Node: card})
		result.Nodes = append(result.Nodes, card)
	}
	result.TotalCount = len(cards)

	result.PageInfo = &model.PageInfo{}
	if result.TotalCount != 0 {
		result.PageInfo.StartCursor = &result.Nodes[0].ID
		result.PageInfo.EndCursor = &result.Nodes[result.TotalCount-1].ID
	}
	result.PageInfo.HasPreviousPage = hasPrevPage
	result.PageInfo.HasNextPage = hasNextPage

	return &result
}

func (s *cardService) GetCardByID(ctx context.Context, id int64) (*model.Card, error) {
	var card repository.Card
	if err := s.db.WithContext(ctx).First(&card, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, goerr.Wrap(err, fmt.Errorf("card not found : %d", id))
		}
		logger.Logger.ErrorContext(ctx, "Failed to get card by ID", err)
		return nil, err
	}
	return ConvertToCard(card), nil
}

func (s *cardService) CreateCard(ctx context.Context, input model.NewCard) (*model.Card, error) {
	gormCard := ConvertToGormCard(input)
	result := s.db.WithContext(ctx).Create(gormCard)
	if result.Error != nil {
		if strings.Contains(result.Error.Error(), "foreign key constraint") {
			return nil, goerr.Wrap(fmt.Errorf("invalid card group ID"))
		}
		return nil, goerr.Wrap(result.Error, fmt.Errorf("failed to create card"))
	}
	return ConvertToCard(*gormCard), nil
}

func (s *cardService) UpdateCard(ctx context.Context, id int64, input model.NewCard) (*model.Card, error) {
	var card repository.Card
	if err := s.db.WithContext(ctx).First(&card, id).Error; err != nil {
		return nil, goerr.Wrap(fmt.Errorf("card does not exist : %d", id))
	}

	card.Front = input.Front
	card.Back = input.Back
	card.ReviewDate = input.ReviewDate
	card.IntervalDays = func() int {
		if input.IntervalDays != nil {
			return *input.IntervalDays
		}
		return card.IntervalDays
	}()
	card.Updated = time.Now().UTC()

	if err := s.db.WithContext(ctx).Save(&card).Error; err != nil {
		return nil, goerr.Wrap(err, "Failed to save card")
	}
	return ConvertToCard(card), nil
}

func (s *cardService) DeleteCard(ctx context.Context, id int64) (*bool, error) {
	result := s.db.WithContext(ctx).Delete(&repository.Card{}, id)
	if result.Error != nil {
		return nil, goerr.Wrap(result.Error, "Failed to delete card")
	}

	success := result.RowsAffected > 0
	if !success {
		return &success, goerr.Wrap(fmt.Errorf("record not found"))
	}

	return &success, nil
}

func (s *cardService) PaginatedCardsByCardGroup(ctx context.Context, cardGroupID int64, first *int, after *int64, last *int, before *int64) (*model.CardConnection, error) {
	var cards []repository.Card
	query := s.db.WithContext(ctx).Where("cardgroup_id = ?", cardGroupID)

	if after != nil {
		query = query.Where("id > ?", *after)
	}
	if before != nil {
		query = query.Where("id < ?", *before)
	}
	if first != nil {
		query = query.Order("id asc").Limit(*first)
	} else if last != nil {
		query = query.Order("id desc").Limit(*last)
	} else {
		query = query.Order("id asc").Limit(s.defaultLimit)
	}

	if err := query.Find(&cards).Error; err != nil {
		return nil, goerr.Wrap(fmt.Errorf("failed to retrieve paginated cards by card group ID"))
	}

	var hasNextPage, hasPrevPage bool
	var count int64

	if len(cards) != 0 {
		startCursor, endCursor := cards[0].ID, cards[len(cards)-1].ID

		err := s.db.WithContext(ctx).Model(&repository.Card{}).
			Where("cardgroup_id = ?", cardGroupID).
			Where("id < ?", startCursor).
			Count(&count).Error
		if err != nil {
			return nil, err
		}
		hasPrevPage = count > 0

		err = s.db.WithContext(ctx).Model(&repository.Card{}).
			Where("cardgroup_id = ?", cardGroupID).
			Where("id > ?", endCursor).
			Count(&count).Error
		if err != nil {
			return nil, err
		}
		hasNextPage = count > 0
	}

	return convertCardConnection(cards, hasPrevPage, hasNextPage), nil
}

func (s *cardService) GetCardsByIDs(ctx context.Context, ids []int64) ([]*model.Card, error) {
	var cards []*repository.Card
	if err := s.db.WithContext(ctx).Where("id IN ?", ids).Find(&cards).Error; err != nil {
		return nil, goerr.Wrap(fmt.Errorf("failed to retrieve cards by IDs"))
	}

	var gqlCards []*model.Card
	for _, card := range cards {
		gqlCards = append(gqlCards, ConvertToCard(*card))
	}

	return gqlCards, nil
}

func (s *cardService) FetchAllCardsByCardGroup(ctx context.Context, cardGroupID int64, first *int) ([]*model.Card, error) {
	var allCards []*model.Card
	var after *int64

	for {
		// Fetch the next batch of cards
		connection, err := s.PaginatedCardsByCardGroup(ctx, cardGroupID, first, after, nil, nil)
		if err != nil {
			return nil, goerr.Wrap(err, "Failed to fetch paginated cards")
		}

		// Append the fetched cards to the result list
		allCards = append(allCards, connection.Nodes...)

		// Check if there are more pages
		if !connection.PageInfo.HasNextPage {
			break
		}

		// Set the cursor for the next batch
		after = connection.PageInfo.EndCursor
	}

	return allCards, nil
}

func (s *cardService) AddNewCards(ctx context.Context, targetCards []model.Card, cardGroupID int64) ([]*model.Card, error) {
	// Use FetchAllCardsByCardGroup to retrieve all cards
	existingCards, err := s.FetchAllCardsByCardGroup(ctx, cardGroupID, nil)
	if err != nil {
		return nil, goerr.Wrap(err)
	}

	// Create a hashmap to manage existing cards by Front value
	existingCardsMap := make(map[string]*model.Card)
	for _, card := range existingCards {
		existingCardsMap[card.Front] = card
	}

	// Slice to hold the modified or newly created cards
	var modifiedCards []*model.Card

	// Process to add or update cards
	for _, targetCard := range targetCards {
		existingCard, exists := existingCardsMap[targetCard.Front]
		if !exists {
			// If Front doesn't match, add as a new card
			newCard := model.NewCard{
				Front:        targetCard.Front,
				Back:         targetCard.Back,
				ReviewDate:   targetCard.ReviewDate,
				IntervalDays: &targetCard.IntervalDays,
				CardgroupID:  targetCard.CardGroupID,
				Created:      time.Now().UTC(),
				Updated:      time.Now().UTC(),
			}
			createdCard, err := s.CreateCard(ctx, newCard)
			if err != nil {
				return nil, goerr.Wrap(err, "Failed to add card")
			}
			modifiedCards = append(modifiedCards, createdCard)
			continue
		}

		// If Front matches, check the similarity of the Back value
		if utils.Similarity(existingCard.Back, targetCard.Back) >= 1.0 {
			// Skip if similarity is 1.0
			continue
		}

		// Update the card if the Back similarity is not 1.0
		newCard := model.NewCard{
			Front:        targetCard.Front,
			Back:         targetCard.Back,
			ReviewDate:   targetCard.ReviewDate,
			IntervalDays: &targetCard.IntervalDays,
			CardgroupID:  targetCard.CardGroupID,
			Created:      time.Now().UTC(),
			Updated:      time.Now().UTC(),
		}
		updatedCard, err := s.UpdateCard(ctx, existingCard.ID, newCard)
		if err != nil {
			return nil, goerr.Wrap(err, "Failed to update card")
		}
		modifiedCards = append(modifiedCards, updatedCard)
	}

	return modifiedCards, nil
}

func (s *cardService) GetCardsByUserAndCardGroup(
	ctx context.Context, cardGroupID int64, order string, limit int) ([]repository.Card, error) {
	var cards []repository.Card

	// Query to find the latest cards with matching user_id and cardgroup_id
	err := s.db.WithContext(ctx).
		Where("cardgroup_id = ?", cardGroupID).
		Order(fmt.Sprintf("updated %s", order)).
		Limit(limit).
		Find(&cards).Error

	if err != nil {
		return nil, goerr.Wrap(err, "Failed to get latest cards by user and card group")
	}

	return cards, nil
}

func (s *cardService) GetRandomCardsFromRecentUpdates(ctx context.Context, cardGroupID int64, limit int, updatedSortOrder string, intervalDaysSortOrder string) ([]model.Card, error) {
	var cards []repository.Card

	// Validate sortOrder for updated and intervalDays
	if updatedSortOrder != repo.ASC && updatedSortOrder != repo.DESC {
		updatedSortOrder = repo.DESC
	}
	if intervalDaysSortOrder != repo.ASC && intervalDaysSortOrder != repo.DESC {
		intervalDaysSortOrder = repo.ASC
	}

	// Query to fetch recent cards by cardGroupID and order them independently by updated and interval_days
	err := s.db.WithContext(ctx).
		Where("cardgroup_id = ?", cardGroupID).
		Order(fmt.Sprintf("updated %s", updatedSortOrder)).
		Order(fmt.Sprintf("interval_days %s", intervalDaysSortOrder)).
		Limit(limit).
		Find(&cards).Error

	if err != nil {
		return nil, goerr.Wrap(err, "Failed to retrieve recent cards")
	}

	// Shuffle the cards if necessary
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	rng.Shuffle(len(cards), func(i, j int) { cards[i], cards[j] = cards[j], cards[i] })

	if len(cards) <= limit {
		return ConvertToCards(cards), nil
	}

	randomCards := cards[:limit]

	return ConvertToCards(randomCards), nil
}
