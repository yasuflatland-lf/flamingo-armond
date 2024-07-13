package flashcard

import (
	"backend/pkg/model"
	"gorm.io/gorm"
	"time"
)

// GetDueCards retrieves flashcards that are due for review
func GetDueCards(db *gorm.DB) ([]model.Card, error) {
	var cards []model.Card
	now := time.Now()
	result := db.Where("review_date <= ?", now).Find(&cards)
	return cards, result.Error
}

// UpdateCardReview updates the review date and interval of a flashcard
func UpdateCardReview(card *model.Card, db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&card, card.ID).Error; err != nil {
			return err
		}
		card.IntervalDays *= 2
		card.ReviewDate = time.Now().AddDate(0, 0, card.IntervalDays)
		if err := tx.Save(card).Error; err != nil {
			return err
		}
		return nil
	})
}

// CreateCardReview creates a new card review entry
func CreateCardReview(card *model.Card, db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(card).Error; err != nil {
			return err
		}
		return nil
	})
}

// MigrateDB performs the database migration for the Card struct
func MigrateDB(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.Cardgroup{},
		&model.Card{},
		&model.User{},
		&model.Role{},
	)
}
