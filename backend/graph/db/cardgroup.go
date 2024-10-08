package db

import (
	customValidator "backend/pkg/validator"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/m-mizutani/goerr"
	"gorm.io/gorm"
)

// BeforeCreate hook to validate the Cardgroup before creating
func (cg *Cardgroup) BeforeCreate(tx *gorm.DB) (err error) {
	return cg.validateAtCreate(cg)
}

// BeforeUpdate hook to validate the Cardgroup before updating
func (cg *Cardgroup) BeforeUpdate(tx *gorm.DB) (err error) {
	return cg.validateStruct(cg)
}

// ValidateStruct function
func (cg *Cardgroup) validateStruct(cardgroup *Cardgroup) error {
	v := customValidator.NewValidateWrapper()
	err := v.Validator().Struct(cardgroup)
	if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			return goerr.Wrap(err, fmt.Sprintf("Field validation for '%s' failed on the '%s' tag", err.Field(), err.Tag()))
		}
	}
	return nil
}

// ValidateName validates only the Name field of the Cardgroup struct
func (cg *Cardgroup) validateAtCreate(cardgroup *Cardgroup) error {
	v := customValidator.NewValidateWrapper()
	err := v.Validator().Var(cardgroup.Name, "required,fl_name,min=1")
	if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			return goerr.Wrap(err, fmt.Sprintf("Field validation for '%s' failed on the '%s' tag", err.Field(), err.Tag()))
		}
	}
	return nil
}
