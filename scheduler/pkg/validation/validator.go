package validation

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// Validator 验证器
var Validator *validator.Validate

// InitValidator 初始化验证器
func InitValidator() {
	Validator = validator.New()
	
	// 注册自定义验证函数
	Validator.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
}

// Validate 验证结构体
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

// formatError 格式化验证错误
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
	default:
		return fmt.Sprintf("%s failed validation on '%s'", field, tag)
	}
}
