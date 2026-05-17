package validation

import (
	"reflect"
	"testing"

	"github.com/go-playground/validator/v10"
)

type TestStruct struct {
	Name  string `json:"name" validate:"required,min=2,max=100"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"min=1,max=120"`
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
	}{
		{
			name: "valid struct",
			input: &TestStruct{
				Name:  "John",
				Email: "john@example.com",
				Age:   25,
			},
			wantErr: false,
		},
		{
			name: "missing required field",
			input: &TestStruct{
				Name:  "",
				Email: "john@example.com",
				Age:   25,
			},
			wantErr: true,
		},
		{
			name: "invalid email",
			input: &TestStruct{
				Name:  "John",
				Email: "invalid-email",
				Age:   25,
			},
			wantErr: true,
		},
		{
			name: "age out of range",
			input: &TestStruct{
				Name:  "John",
				Email: "john@example.com",
				Age:   150,
			},
			wantErr: true,
		},
		{
			name:    "nil input",
			input:   nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInitValidator(t *testing.T) {
	Validator = nil

	InitValidator()

	if Validator == nil {
		t.Error("Validator should not be nil after initialization")
	}
}

func TestFormatError_WithRealValidator(t *testing.T) {
	InitValidator()

	tests := []struct {
		name        string
		input       interface{}
		expectedMsg string
	}{
		{
			name: "required field error",
			input: &TestStruct{
				Name:  "",
				Email: "john@example.com",
				Age:   25,
			},
			expectedMsg: "validation failed: name is required",
		},
		{
			name: "invalid email error",
			input: &TestStruct{
				Name:  "John",
				Email: "invalid",
				Age:   25,
			},
			expectedMsg: "validation failed: email must be a valid email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.input)

			if err == nil {
				t.Error("expected error but got nil")
				return
			}

			if err.Error() != tt.expectedMsg {
				t.Errorf("expected error message %q, got %q", tt.expectedMsg, err.Error())
			}
		})
	}
}

type mockFieldError struct {
	field     string
	tag       string
	param     string
	actualTag string
}

func (e *mockFieldError) Field() string {
	return e.field
}

func (e *mockFieldError) Tag() string {
	return e.tag
}

func (e *mockFieldError) ActualTag() string {
	return e.actualTag
}

func (e *mockFieldError) Param() string {
	return e.param
}

func (e *mockFieldError) Namespace() string {
	return ""
}

func (e *mockFieldError) StructNamespace() string {
	return ""
}

func (e *mockFieldError) StructField() string {
	return ""
}

func (e *mockFieldError) Value() interface{} {
	return nil
}

func (e *mockFieldError) Kind() reflect.Kind {
	return reflect.String
}

func (e *mockFieldError) Type() string {
	return ""
}

func (e *mockFieldError) Error() string {
	return ""
}

func (e *mockFieldError) Translate(ut interface{}) string {
	return ""
}

func (e *mockFieldError) Dive() []validator.FieldError {
	return nil
}