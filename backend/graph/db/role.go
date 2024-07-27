package db

import (
	customValidator "backend/pkg/validator"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"time"
)

func (r *Role) BeforeSave(tx *gorm.DB) (err error) {
	return r.validateStruct(r)
}

// ValidateStruct 関数
func (r *Role) validateStruct(role *Role) error {
	v := customValidator.NewValidateWrapper()
	err := v.Validator().Struct(role)
	if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			return errors.New(fmt.Sprintf("Field validation for '%s' failed on the '%s' tag", err.Field(), err.Tag()))
		}
	}
	return nil
}

// Updating data in same transaction
func (r *Role) AfterUpdate(tx *gorm.DB) (err error) {
	tx.Model(&Role{}).Where("id = ?", r.ID).Update("updated", time.Now())
	return nil
}
