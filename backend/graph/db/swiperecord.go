package db

import (
	customValidator "backend/pkg/validator"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/m-mizutani/goerr"
	"gorm.io/gorm"
	"time"
)

// BeforeCreate hook to validate the SwipeRecord fields
func (s *SwipeRecord) BeforeCreate(tx *gorm.DB) (err error) {
	return s.validateAtCreate(s)
}

// BeforeUpdate hook to validate the SwipeRecord fields
func (s *SwipeRecord) BeforeUpdate(tx *gorm.DB) (err error) {
	return s.validateStruct(s)
}

// validateStruct validates the entire SwipeRecord struct
func (s *SwipeRecord) validateStruct(swipeRecord *SwipeRecord) error {
	v := customValidator.NewValidateWrapper()
	err := v.Validator().Struct(swipeRecord)
	if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			return goerr.Wrap(err, fmt.Sprintf("Field validation for '%s' failed on the '%s' tag", err.Field(), err.Tag()))
		}
	}
	return nil
}

// validateAtCreate validates specific fields during the creation of a SwipeRecord
func (s *SwipeRecord) validateAtCreate(swipeRecord *SwipeRecord) error {
	v := customValidator.NewValidateWrapper()

	err := v.Validator().Var(swipeRecord.UserID, "required")
	if err != nil {
		return goerr.Wrap(err, fmt.Sprintf("Field validation for 'user_id' failed %+v", err))
	}

	err = v.Validator().Var(swipeRecord.Direction, "required,oneof=left right up down") // Assuming 'Direction' has specific allowed values
	if err != nil {
		return goerr.Wrap(err, fmt.Sprintf("Field validation for 'direction' failed %+v", err))
	}

	return nil
}

// AfterUpdate hook to update the timestamp in the same transaction
func (s *SwipeRecord) AfterUpdate(tx *gorm.DB) (err error) {
	tx.Model(&SwipeRecord{}).Where("id = ?", s.ID).Update("updated", time.Now().UTC())
	return nil
}
