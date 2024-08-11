package db

import (
	customValidator "backend/pkg/validator"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/m-mizutani/goerr"
	"gorm.io/gorm"
	"time"
)

// BeforeCreate hook to validate the Role before creating
func (r *Role) BeforeCreate(tx *gorm.DB) (err error) {
	return r.validateAtCreate(r)
}

func (r *Role) BeforeUpdate(tx *gorm.DB) (err error) {
	return r.validateStruct(r)
}

func (r *Role) validateStruct(role *Role) error {
	v := customValidator.NewValidateWrapper()
	err := v.Validator().Struct(role)
	if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			return goerr.Wrap(err, fmt.Errorf("Field validation for '%s' failed on the '%s' tag", err.Field(), err.Tag()))
		}
	}
	return nil
}

// ValidateNameAtCreate validates only the Name field of the Role struct
func (r *Role) validateAtCreate(role *Role) error {
	v := customValidator.NewValidateWrapper()
	err := v.Validator().Var(role.Name, "required,fl_name,min=1")
	if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			return goerr.Wrap(err, fmt.Errorf("Field validation for '%s' failed on the '%s' tag", err.Field(), err.Tag()))
		}
	}
	return nil
}

// Updating data in same transaction
func (r *Role) AfterUpdate(tx *gorm.DB) (err error) {
	tx.Model(&Role{}).Where("id = ?", r.ID).Update("updated", time.Now().UTC())
	return nil
}
