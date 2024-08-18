package db

import (
	customValidator "backend/pkg/validator"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/m-mizutani/goerr"
	"gorm.io/gorm"
)

func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	return u.validateAtCreate(u)
}

func (u *User) BeforeUpdate(tx *gorm.DB) (err error) {
	return u.validateStruct(u)
}

func (u *User) validateStruct(user *User) error {
	v := customValidator.NewValidateWrapper()
	err := v.Validator().Struct(user)
	if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			return goerr.Wrap(err, fmt.Sprintf("Field validation for '%s' failed on the '%s' tag", err.Field(), err.Tag()))
		}
	}
	return nil
}

// ValidateName validates only the Name field of the User struct
func (u *User) validateAtCreate(user *User) error {
	v := customValidator.NewValidateWrapper()
	err := v.Validator().Var(user.Name, "required,fl_name,min=1")
	if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			return goerr.Wrap(err, fmt.Sprintf("Field validation for '%s' failed on the '%s' tag", err.Field(), err.Tag()))
		}
	}
	return nil
}
