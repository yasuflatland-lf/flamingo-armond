package db

import (
	customValidator "backend/pkg/validator"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"time"
)

func (c *Card) BeforeSave(tx *gorm.DB) (err error) {

	return c.validateStruct(c)
}

// ValidateStruct 関数
func (c *Card) validateStruct(card *Card) error {
	v := customValidator.NewValidateWrapper()
	err := v.Validator().Struct(card)
	if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			return errors.New(fmt.Sprintf("Field validation for '%s' failed on the '%s' tag", err.Field(), err.Tag()))
		}
	}
	return nil
}

// Updating data in same transaction
func (c *Card) AfterUpdate(tx *gorm.DB) (err error) {

	tx.Model(&Card{}).Where("id = ?", c.ID).Update("updated", time.Now())

	return nil
}
