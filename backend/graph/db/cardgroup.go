package db

import (
	customValidator "backend/pkg/validator"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"time"
)

func (cg *Cardgroup) BeforeSave(tx *gorm.DB) (err error) {
	return cg.validateStruct(cg)
}

// ValidateStruct 関数
func (cg *Cardgroup) validateStruct(cardgroup *Cardgroup) error {
	v := customValidator.NewValidateWrapper()
	err := v.Validator().Struct(cardgroup)
	if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			return errors.New(fmt.Sprintf("Field validation for '%s' failed on the '%s' tag", err.Field(), err.Tag()))
		}
	}
	return nil
}

// Updating data in same transaction
func (cg *Cardgroup) AfterUpdate(tx *gorm.DB) (err error) {
	tx.Model(&Cardgroup{}).Where("id = ?", cg.ID).Update("updated", time.Now())
	return nil
}
