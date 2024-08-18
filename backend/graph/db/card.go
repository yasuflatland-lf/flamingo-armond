package db

import (
	customValidator "backend/pkg/validator"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/m-mizutani/goerr"
	"gorm.io/gorm"
)

// BeforeCreate hook to validate the front and back fields
func (c *Card) BeforeCreate(tx *gorm.DB) (err error) {
	return c.validateAtCreate(c)
}

// BeforeUpdate hook
func (c *Card) BeforeUpdate(tx *gorm.DB) (err error) {
	return c.validateStruct(c)
}

func (c *Card) validateStruct(card *Card) error {
	v := customValidator.NewValidateWrapper()
	err := v.Validator().Struct(card)
	if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			return goerr.Wrap(err, fmt.Sprintf("Field validation for '%s' failed on the '%s' tag", err.Field(), err.Tag()))
		}
	}
	return nil
}

// ValidateFrontAndBack function to validate only front and back fields
func (c *Card) validateAtCreate(card *Card) error {
	v := customValidator.NewValidateWrapper()
	err := v.Validator().Var(card.Front, "required,min=1") // Assuming you want to check if 'front' is required
	if err != nil {
		return goerr.Wrap(err, fmt.Sprintf("Field validation for 'front' failed %+v", err))
	}

	err = v.Validator().Var(card.Back, "required,min=1") // Assuming you want to check if 'back' is required
	if err != nil {
		return goerr.Wrap(err, fmt.Sprintf("Field validation for 'back' failed %+v", err))
	}

	return nil
}
