package usecases

import (
	"backend/graph/model"
	"backend/graph/services"
	"backend/pkg/textdic"
	"context"
	"fmt"
)

type dictionaryManagerUsecase struct {
	cardService           services.CardService          // Pointer to CardService
	textDictionaryService textdic.TextDictionaryService // Pointer to textDictionaryService
}

func NewDictionaryManagerUsecase(cardService services.CardService, textDictionaryService textdic.TextDictionaryService) *dictionaryManagerUsecase {
	return &dictionaryManagerUsecase{
		cardService:           cardService,
		textDictionaryService: textDictionaryService,
	}
}

// UpsertCards decodes a base64 encoded dictionary, processes it, and creates cards from it.
func (dmu *dictionaryManagerUsecase) UpsertCards(ctx context.Context, encodedDictionary string, cardGroupID int64) ([]*model.Card, error) {

	// Decode the base64 encoded dictionary
	decodedDictionary, err := dmu.textDictionaryService.DecodeBase64(encodedDictionary)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 dictionary: %+v", err)
	}

	// Process the decoded dictionary to get nodes
	nodes, errs := dmu.textDictionaryService.Process(decodedDictionary)
	if len(errs) > 0 {
		return nil, fmt.Errorf("failed to process dictionary: %+v", errs)
	}

	// Convert nodes to []model.Card
	var cards []model.Card
	for _, node := range nodes {
		card := model.Card{
			Front: node.Word,
			Back:  node.Definition,
			// Additional fields like ReviewDate, IntervalDays, etc. can be set here as needed.
		}
		cards = append(cards, card)
	}

	// Use AddNewCards to add the generated cards to the card service
	createdCards, err := dmu.cardService.AddNewCards(ctx, cards, cardGroupID)
	if err != nil {
		return nil, fmt.Errorf("failed to add new cards: %w", err)
	}

	return createdCards, nil
}
