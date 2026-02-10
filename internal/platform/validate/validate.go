package validate

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// FieldError represents a single field validation failure.
type FieldError struct {
	Field   string
	Message string
	Value   string
}

// ValidationError is returned when input validation fails.
type ValidationError struct {
	Message string
	Fields  []FieldError
}

func (e *ValidationError) Error() string {
	return e.Message
}

// AppValidator wraps go-playground/validator for Echo's Validator interface.
type AppValidator struct {
	v *validator.Validate
}

// New creates a new AppValidator.
func New() *AppValidator {
	v := validator.New()

	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		if name := tagName(fld, "json"); name != "" {
			return name
		}
		if name := tagName(fld, "query"); name != "" {
			return name
		}
		if name := tagName(fld, "param"); name != "" {
			return name
		}
		return fld.Name
	})

	return &AppValidator{v: v}
}

// Validate validates the given struct and returns a *ValidationError on failure.
func (av *AppValidator) Validate(i any) error {
	err := av.v.Struct(i)
	if err == nil {
		return nil
	}

	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		fields := make([]FieldError, len(ve))
		for idx, fe := range ve {
			fields[idx] = FieldError{
				Field:   fe.Field(),
				Message: buildMessage(fe),
				Value:   fmt.Sprintf("%v", fe.Value()),
			}
		}
		return &ValidationError{
			Message: "validation failed",
			Fields:  fields,
		}
	}

	return &ValidationError{Message: err.Error()}
}

func tagName(fld reflect.StructField, tag string) string {
	name, _, _ := strings.Cut(fld.Tag.Get(tag), ",")
	if name == "" || name == "-" {
		return ""
	}
	return name
}

func buildMessage(fe validator.FieldError) string {
	field := fe.Field()
	switch fe.Tag() {
	case "required":
		return field + " is required"
	case "min":
		return field + " must be at least " + fe.Param()
	case "max":
		return field + " must be at most " + fe.Param()
	case "email":
		return field + " must be a valid email address"
	case "e164":
		return field + " must be a valid E.164 phone number"
	case "oneof":
		return field + " must be one of: " + fe.Param()
	default:
		return field + " failed on " + fe.Tag() + " validation"
	}
}
