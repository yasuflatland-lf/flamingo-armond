package validator

import (
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"regexp"
	"time"
)

type validateWrapper struct {
	ValidatorInstance *validator.Validate
}

type ValidateWrapper interface {
	Validator() *validator.Validate
	ValidateStruct(*interface{}) error
}

func NewValidateWrapper(options ...validator.Option) ValidateWrapper {

	// Add the WithRequiredStructEnabled option to the provided options
	options = append(options, validator.WithRequiredStructEnabled())

	validatorInstance := validator.New(options...)

	wrapper := &validateWrapper{
		ValidatorInstance: validatorInstance,
	}

	validatorInstance.RegisterValidation("fl_id", wrapper.IDValidation)
	validatorInstance.RegisterValidation("fl_name", wrapper.NameValidation)
	validatorInstance.RegisterValidation("fl_datetime", wrapper.DatetimeValidation)
	return wrapper
}

func (v *validateWrapper) Validator() *validator.Validate {
	return v.ValidatorInstance
}

func (v *validateWrapper) ValidateStruct(s *interface{}) error {
	err := v.ValidatorInstance.Struct(s)
	if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			return errors.New(fmt.Sprintf("Field validation for '%s' failed on the '%s' tag", err.Field(), err.Tag()))
		}
	}
	return nil
}

// User, Role name validation
func (v *validateWrapper) NameValidation(fl validator.FieldLevel) bool {
	name := fl.Field().String()

	// Define a regular expression to match unwanted Unicode categories
	// Refer https://www.tohoho-web.com/ex/regexp.html#unicode Properties section.
	re := regexp.MustCompile(`^[^!:;]+$`)
	if re.MatchString(name) {
		return true
	}

	return false
}

func (v *validateWrapper) DatetimeValidation(fl validator.FieldLevel) bool {
	dateTime, ok := fl.Field().Interface().(time.Time)
	if !ok {
		return false
	}

	// Example: Ensure the time is not zero (i.e., it has been set)
	if dateTime.IsZero() {
		return false
	}

	return true
}

func (v *validateWrapper) IDValidation(fl validator.FieldLevel) bool {
	intValue, ok := fl.Field().Interface().(int64)
	if !ok {
		return false
	}

	if intValue < 0 {
		return false
	}

	return true
}
