package services

import (
	repository "backend/graph/db"
	"backend/graph/model"
	repo "backend/pkg/repository"
	"context"
	"fmt"
	"time"

	"github.com/m-mizutani/goerr"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// cardGroupService provides methods to manage card groups in the database.
type cardGroupService struct {
	db           *gorm.DB
	defaultLimit int
}

type CardGroupService interface {
	GetCardGroupByID(ctx context.Context, id int64) (*model.CardGroup, error)
	CreateCardGroup(ctx context.Context, input model.NewCardGroup) (*model.CardGroup, error)
	CardGroups(ctx context.Context) ([]*model.CardGroup, error)
	UpdateCardGroup(ctx context.Context, id int64, input model.NewCardGroup) (*model.CardGroup, error)
	DeleteCardGroup(ctx context.Context, id int64) (*bool, error)
	AddUserToCardGroup(ctx context.Context, userID int64, cardGroupID int64) (*model.CardGroup, error)
	RemoveUserFromCardGroup(ctx context.Context, userID int64, cardGroupID int64) (*model.CardGroup, error)
	GetCardGroupsByUser(ctx context.Context, userID int64) ([]*model.CardGroup, error)
	PaginatedCardGroupsByUser(ctx context.Context, userID int64, first *int, after *int64, last *int, before *int64) (*model.CardGroupConnection, error)
	GetCardGroupsByIDs(ctx context.Context, ids []int64) ([]*model.CardGroup, error)
	UpdateCardGroupUserState(ctx context.Context, cardGroupID int64, userID int64, newState int) error
	GetLatestCardgroupUsers(ctx context.Context, cardGroupID int64, limit int, sortOrder string) ([]*repository.CardgroupUser, error)
	GetCardgroupUser(ctx context.Context, cardGroupID int64, userID int64) (*repository.CardgroupUser, error)
}

// NewCardGroupService creates a new CardGroupService instance.
func NewCardGroupService(db *gorm.DB, defaultLimit int) CardGroupService {
	return &cardGroupService{db: db, defaultLimit: defaultLimit}
}

// ConvertToGormCardGroupFromNew converts a NewCardGroup input to a GORM-compatible Cardgroup model.
func ConvertToGormCardGroupFromNew(input model.NewCardGroup) *repository.Cardgroup {
	return &repository.Cardgroup{
		Name:    input.Name,
		Created: time.Now().UTC(),
		Updated: time.Now().UTC(),
	}
}

// ConvertToCardGroup converts a Cardgroup repository model to a GraphQL-compatible CardGroup model.
func ConvertToCardGroup(cardGroup repository.Cardgroup) *model.CardGroup {
	return &model.CardGroup{
		ID:      cardGroup.ID,
		Name:    cardGroup.Name,
		Created: cardGroup.Created,
		Updated: cardGroup.Updated,
	}
}

// GetCardGroupByID retrieves a card group by its ID from the database.
func (s *cardGroupService) GetCardGroupByID(ctx context.Context, id int64) (*model.CardGroup, error) {
	var cardGroup repository.Cardgroup
	if err := s.db.First(&cardGroup, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, goerr.Wrap(err, fmt.Errorf("card group not found : %d", id))
		}
		return nil, goerr.Wrap(err, "failed to retrieve card group by ID")
	}
	return ConvertToCardGroup(cardGroup), nil
}

// CreateCardGroup creates a new card group in the database.
func (s *cardGroupService) CreateCardGroup(ctx context.Context, input model.NewCardGroup) (*model.CardGroup, error) {
	gormCardGroup := ConvertToGormCardGroupFromNew(input)
	result := s.db.WithContext(ctx).Create(&gormCardGroup)
	if result.Error != nil {
		return nil, goerr.Wrap(result.Error, "failed to create card group")
	}
	return ConvertToCardGroup(*gormCardGroup), nil
}

// CardGroups retrieves all card groups from the database.
func (s *cardGroupService) CardGroups(ctx context.Context) ([]*model.CardGroup, error) {
	var cardGroups []repository.Cardgroup
	if err := s.db.WithContext(ctx).Find(&cardGroups).Error; err != nil {
		return nil, goerr.Wrap(err, "failed to retrieve card groups")
	}
	var gqlCardGroups []*model.CardGroup
	for _, cardGroup := range cardGroups {
		gqlCardGroups = append(gqlCardGroups, ConvertToCardGroup(cardGroup))
	}
	return gqlCardGroups, nil
}

// UpdateCardGroup updates a card group in the database by its ID.
func (s *cardGroupService) UpdateCardGroup(ctx context.Context, id int64, input model.NewCardGroup) (*model.CardGroup, error) {
	var cardGroup repository.Cardgroup
	if err := s.db.WithContext(ctx).First(&cardGroup, id).Error; err != nil {
		return nil, goerr.Wrap(err, fmt.Errorf("card group not found for update : %d", id))
	}
	cardGroup.Name = input.Name
	cardGroup.Updated = time.Now().UTC()
	if err := s.db.WithContext(ctx).Save(&cardGroup).Error; err != nil {
		return nil, goerr.Wrap(err, "failed to update card group")
	}
	return ConvertToCardGroup(cardGroup), nil
}

// DeleteCardGroup deletes a card group from the database by its ID.
func (s *cardGroupService) DeleteCardGroup(ctx context.Context, id int64) (*bool, error) {
	success := false
	result := s.db.WithContext(ctx).Delete(&repository.Cardgroup{}, id)
	if result.Error != nil {
		return &success, goerr.Wrap(result.Error, "failed to delete card group")
	}
	if result.RowsAffected == 0 {
		return &success, goerr.Wrap(fmt.Errorf("no card group found for deletion"))
	}
	success = true
	return &success, nil
}

// AddUserToCardGroup adds a user to a card group in the database.
func (s *cardGroupService) AddUserToCardGroup(ctx context.Context, userID int64, cardGroupID int64) (*model.CardGroup, error) {
	var user repository.User
	var cardGroup repository.Cardgroup
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		return nil, goerr.Wrap(err, fmt.Errorf("user not found : %d", userID))
	}
	if err := s.db.WithContext(ctx).First(&cardGroup, cardGroupID).Error; err != nil {
		return nil, goerr.Wrap(err, fmt.Errorf("card group not found : %d", cardGroupID))
	}
	if err := s.db.Model(&cardGroup).Association("Users").Append(&user); err != nil {
		return nil, goerr.Wrap(err, "failed to add user to card group")
	}
	return ConvertToCardGroup(cardGroup), nil
}

// RemoveUserFromCardGroup removes a user from a card group in the database.
func (s *cardGroupService) RemoveUserFromCardGroup(ctx context.Context, userID int64, cardGroupID int64) (*model.CardGroup, error) {
	var user repository.User
	var cardGroup repository.Cardgroup
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		return nil, goerr.Wrap(err, fmt.Errorf("user not found : %d", userID))
	}
	if err := s.db.WithContext(ctx).First(&cardGroup, cardGroupID).Error; err != nil {
		return nil, goerr.Wrap(err, fmt.Errorf("card group not found : %d", cardGroupID))
	}
	if err := s.db.Model(&cardGroup).Association("Users").Delete(&user); err != nil {
		return nil, goerr.Wrap(err, "failed to remove user from card group")
	}
	return ConvertToCardGroup(cardGroup), nil
}

// GetCardGroupsByUser retrieves all card groups associated with a specific user from the database.
func (s *cardGroupService) GetCardGroupsByUser(ctx context.Context, userID int64) ([]*model.CardGroup, error) {
	var user repository.User
	if err := s.db.WithContext(ctx).Preload("CardGroups").First(&user, userID).Error; err != nil {
		return nil, goerr.Wrap(err, fmt.Errorf("failed to get card groups by user ID : %d", userID))
	}
	var gqlCardGroups []*model.CardGroup
	for _, group := range user.CardGroups {
		gqlCardGroups = append(gqlCardGroups, ConvertToCardGroup(group))
	}
	return gqlCardGroups, nil
}

// PaginatedCardGroupsByUser retrieves a paginated list of card groups associated with a specific user.
func (s *cardGroupService) PaginatedCardGroupsByUser(ctx context.Context, userID int64, first *int, after *int64, last *int, before *int64) (*model.CardGroupConnection, error) {
	var user repository.User
	var cardGroups []repository.Cardgroup

	query := s.db.WithContext(ctx).Model(&user).Where("id = ?", userID).Preload("CardGroups", func(db *gorm.DB) *gorm.DB {
		if after != nil {
			db = db.Where("cardgroups.id > ?", *after)
		}
		if before != nil {
			db = db.Where("cardgroups.id < ?", *before)
		}
		if first != nil {
			db = db.Order("cardgroups.id asc").Limit(*first)
		} else if last != nil {
			db = db.Order("cardgroups.id desc").Limit(*last)
		} else {
			db = db.Order("cardgroups.id asc").Limit(s.defaultLimit)
		}
		return db
	})

	if err := query.Find(&user).Error; err != nil {
		return nil, goerr.Wrap(err, "failed to get paginated card groups by user")
	}

	cardGroups = user.CardGroups

	var edges []*model.CardGroupEdge
	var nodes []*model.CardGroup
	for _, cardGroup := range cardGroups {
		node := ConvertToCardGroup(cardGroup)
		edges = append(edges, &model.CardGroupEdge{
			Cursor: cardGroup.ID,
			Node:   node,
		})
		nodes = append(nodes, node)
	}

	pageInfo := &model.PageInfo{}
	if len(cardGroups) > 0 {
		pageInfo.HasNextPage = len(cardGroups) == s.defaultLimit
		pageInfo.HasPreviousPage = len(cardGroups) == s.defaultLimit
		if len(edges) > 0 {
			pageInfo.StartCursor = &edges[0].Cursor
			pageInfo.EndCursor = &edges[len(edges)-1].Cursor
		}
	}

	return &model.CardGroupConnection{
		Edges:      edges,
		Nodes:      nodes,
		PageInfo:   pageInfo,
		TotalCount: len(cardGroups),
	}, nil
}

// GetCardGroupsByIDs retrieves card groups by their IDs from the database.
func (s *cardGroupService) GetCardGroupsByIDs(ctx context.Context, ids []int64) ([]*model.CardGroup, error) {
	var cardGroups []*repository.Cardgroup
	if err := s.db.WithContext(ctx).Where("id IN ?", ids).Find(&cardGroups).Error; err != nil {
		return nil, goerr.Wrap(err, "failed to retrieve card groups by IDs")
	}

	var gqlCardGroups []*model.CardGroup
	for _, cardGroup := range cardGroups {
		gqlCardGroups = append(gqlCardGroups, ConvertToCardGroup(*cardGroup))
	}

	return gqlCardGroups, nil
}

// UpdateCardGroupUserState updates the state of a user in a specific card group in the database.
func (s *cardGroupService) UpdateCardGroupUserState(ctx context.Context, cardGroupID int64, userID int64, newState int) error {
	// Initialize the structure with the IDs and the new state
	cardGroupUser := repository.CardgroupUser{
		CardGroupID: cardGroupID,
		UserID:      userID,
		State:       newState,
		Updated:     time.Now().UTC(),
	}

	// Perform the update using Gorm
	result := s.db.WithContext(ctx).
		Model(&repository.CardgroupUser{}).
		Where("cardgroup_id = ? AND user_id = ?", cardGroupID, userID).
		Updates(map[string]interface{}{
			"state":   cardGroupUser.State,
			"updated": cardGroupUser.Updated,
		})

	if result.Error != nil {
		return goerr.Wrap(result.Error)
	}

	return nil
}

func (s *cardGroupService) GetLatestCardgroupUsers(ctx context.Context, cardGroupID int64, limit int, sortOrder string) ([]*repository.CardgroupUser, error) {
	var cardgroupUsers []*repository.CardgroupUser

	// Default to "DESC" if the provided sortOrder is empty or invalid
	if sortOrder != repo.ASC && sortOrder != repo.DESC {
		sortOrder = repo.DESC
	}

	// Query the CardgroupUser records by CardGroupID with the specified limit and order
	if err := s.db.WithContext(ctx).
		Where("cardgroup_id = ?", cardGroupID).
		Order("updated " + sortOrder).
		Limit(limit).
		Find(&cardgroupUsers).Error; err != nil {
		return nil, goerr.Wrap(err, "failed to retrieve CardgroupUsers")
	}

	return cardgroupUsers, nil
}

func (s *cardGroupService) GetCardgroupUser(ctx context.Context, cardGroupID int64, userID int64) (*repository.CardgroupUser, error) {
	var cardgroupUser repository.CardgroupUser
	if err := s.db.WithContext(ctx).
		Where("cardgroup_id = ? AND user_id = ?", cardGroupID, userID).
		First(&cardgroupUser).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, goerr.Wrap(err, fmt.Errorf("cardgroup user not found for cardGroupID: %d, userID: %d", cardGroupID, userID))
		}
		return nil, goerr.Wrap(err, "failed to retrieve cardgroup user")
	}
	return &cardgroupUser, nil
}
