package dictionary_manager

import (
	"backend/graph/model"
	"backend/graph/services"
	"backend/pkg/textdic"
	"context"
	"fmt"
	"github.com/m-mizutani/goerr"
	"time"
)

type DictionaryManagerUsecase interface {
	UpsertCards(ctx context.Context, encodedDictionary string, cardGroupID int64) ([]*model.Card, error)
}

type dictionaryManagerUsecase struct {
	cardService           services.CardService          // Pointer to CardService
	textDictionaryService textdic.TextDictionaryService // Pointer to textDictionaryService
}

func NewDictionaryManagerUsecase(cardService services.CardService, textDictionaryService textdic.TextDictionaryService) DictionaryManagerUsecase {
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
		return nil, goerr.Wrap(err, "failed to decode base64 dictionary")
	}

	// Process the decoded dictionary to get nodes
	nodes, errs := dmu.textDictionaryService.Process(decodedDictionary)
	if len(errs) > 0 {
		return nil, goerr.Wrap(fmt.Errorf("failed to process dictionary: %+v", errs))
	}

	var cards []model.Card
	for _, node := range nodes {
		card := model.Card{
			Front:        node.Word,
			Back:         node.Definition,
			ReviewDate:   time.Now().UTC(),
			IntervalDays: 1,
			Created:      time.Now().UTC(),
			Updated:      time.Now().UTC(),
			CardGroupID:  cardGroupID,
			CardGroup:    nil, // Assuming this will be populated later or left nil
		}
		cards = append(cards, card)
	}

	// Use AddNewCards to add the generated cards to the card service
	createdCards, err := dmu.cardService.AddNewCards(ctx, cards, cardGroupID)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to add new cards")
	}

	return createdCards, nil
}
