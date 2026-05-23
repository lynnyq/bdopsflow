package validation

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

var Validator *validator.Validate

func InitValidator() {
	Validator = validator.New()

	Validator.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	Validator.RegisterValidation("regexp", func(fl validator.FieldLevel) bool {
		param := fl.Param()
		if param == "" {
			return true
		}
		re, err := regexp.Compile("^" + param + "$")
		if err != nil {
			return false
		}
		return re.MatchString(fl.Field().String())
	})
}

func Validate(s interface{}) error {
	if Validator == nil {
		InitValidator()
	}
	err := Validator.Struct(s)
	if err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			var errs []string
			for _, e := range validationErrors {
				errs = append(errs, formatError(e))
			}
			return fmt.Errorf("validation failed: %s", strings.Join(errs, "; "))
		}
		return err
	}
	return nil
}

func formatError(e validator.FieldError) string {
	field := e.Field()
	tag := e.Tag()
	param := e.Param()

	switch tag {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "email":
		return fmt.Sprintf("%s must be a valid email", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", field, param)
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", field, param)
	case "len":
		return fmt.Sprintf("%s must be exactly %s characters", field, param)
	case "alpha":
		return fmt.Sprintf("%s must contain only letters", field)
	case "alphanum":
		return fmt.Sprintf("%s must contain only letters and numbers", field)
	case "regexp":
		return fmt.Sprintf("%s format is invalid", field)
	default:
		return fmt.Sprintf("%s failed validation on '%s'", field, tag)
	}
}
