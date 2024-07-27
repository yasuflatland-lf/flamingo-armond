package db

import (
	customValidator "backend/pkg/validator"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"time"
)

func (u *User) BeforeSave(tx *gorm.DB) (err error) {
	return u.validateStruct(u)
}

// ValidateStruct 関数
func (u *User) validateStruct(user *User) error {
	v := customValidator.NewValidateWrapper()
	err := v.Validator().Struct(user)
	if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			return errors.New(fmt.Sprintf("Field validation for '%s' failed on the '%s' tag", err.Field(), err.Tag()))
		}
	}
	return nil
}

// Updating data in same transaction
func (u *User) AfterUpdate(tx *gorm.DB) (err error) {
	tx.Model(&User{}).Where("id = ?", u.ID).Update("updated", time.Now())
	return nil
}
