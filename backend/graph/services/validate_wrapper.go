package services

import (
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
}

func NewValidateWrapper(options ...validator.Option) ValidateWrapper {

	// Add the WithRequiredStructEnabled option to the provided options
	options = append(options, validator.WithRequiredStructEnabled())

	validatorInstance := validator.New(options...)

	wrapper := &validateWrapper{
		ValidatorInstance: validatorInstance,
	}

	validatorInstance.RegisterValidation("fl_name", wrapper.NameValidation)
	validatorInstance.RegisterValidation("fl_datetime", wrapper.DatetimeValidation)
	return wrapper
}

func (v *validateWrapper) Validator() *validator.Validate {
	return v.ValidatorInstance
}

// User, Role name validaion
func (v *validateWrapper) NameValidation(fl validator.FieldLevel) bool {
	name := fl.Field().String()

	// Define a regular expression to match unwanted Unicode categories
	// Refer https://www.tohoho-web.com/ex/regexp.html#unicode Properties section.
	re := regexp.MustCompile(`[\p{Cc}\p{Cf}\p{Co}\p{Cs}\p{Pc}\p{Pd}\p{Pe}\p{Pf}\p{Pi}\p{Po}\p{Ps}\p{Mc}\p{Me}\p{Mn}\p{Sc}\p{Sm}\p{Zl}\p{Zp}]`)
	if re.MatchString(name) {
		return false
	}

	return true
}

func (v *validateWrapper) DatetimeValidation(fl validator.FieldLevel) bool {

	_, err := time.Parse(time.RFC3339, fl.Field().String())
	fmt.Println(err)
	if err != nil {
		return false
	}
	return true
}
