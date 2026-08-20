package validate

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/go-playground/validator/v10"
)

// FieldError represents a single field validation failure.
type FieldError struct {
	Field    string
	Message  string
	Location Location
}

// Location identifies an application-owned input structure without carrying a rejected value.
type Location uint8

const (
	LocationNone Location = iota
	LocationBody
	LocationQuery
	LocationHeader
	LocationPath
)

// ValidationError is returned when input validation fails.
type ValidationError struct {
	Message string
	Fields  []FieldError
}

func (e *ValidationError) Error() string {
	return e.Message
}

// StatusCode exposes the response status to Echo middleware and error handlers.
func (e *ValidationError) StatusCode() int {
	return http.StatusUnprocessableEntity
}

// AppValidator wraps go-playground/validator for Echo's Validator interface.
type AppValidator struct {
	v *validator.Validate
}

// New creates a new AppValidator.
func New() *AppValidator {
	v := validator.New(validator.WithRequiredStructEnabled())
	if err := v.RegisterValidation("bounded_name", validateBoundedName); err != nil {
		panic(err)
	}
	if err := v.RegisterValidation("contact_email", validateContactEmail); err != nil {
		panic(err)
	}
	if err := v.RegisterValidation("phone_number", validatePhoneNumber); err != nil {
		panic(err)
	}

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

	if ve, ok := errors.AsType[validator.ValidationErrors](err); ok {
		fields := make([]FieldError, len(ve))
		for idx, fe := range ve {
			fields[idx] = FieldError{
				Field:    fe.Field(),
				Message:  buildMessage(fe),
				Location: fieldLocation(reflect.TypeOf(i), fe.StructField()),
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
	case "bounded_name":
		return field + " must be a valid name"
	case "contact_email":
		return field + " must be a valid contact email"
	case "phone_number":
		return field + " must be a valid E.164 phone number"
	case "eq":
		return field + " must be " + fe.Param()
	default:
		return field + " is invalid"
	}
}

func fieldLocation(typ reflect.Type, structField string) Location {
	if typ == nil {
		return LocationNone
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
		if typ == nil {
			return LocationNone
		}
	}
	if typ.Kind() != reflect.Struct {
		return LocationNone
	}
	field, ok := typ.FieldByName(structField)
	if !ok {
		return LocationNone
	}
	switch {
	case tagName(field, "json") != "":
		return LocationBody
	case tagName(field, "query") != "":
		return LocationQuery
	case tagName(field, "header") != "":
		return LocationHeader
	case tagName(field, "param") != "":
		return LocationPath
	default:
		return LocationNone
	}
}

func validateBoundedName(fl validator.FieldLevel) bool {
	return BoundedName(fl.Field().String())
}

// BoundedName applies the portable scalar and whitespace definition.
func BoundedName(value string) bool {
	if !utf8.ValidString(value) {
		return false
	}
	count := utf8.RuneCountInString(value)
	if count < 1 || count > 100 || hasPortableWhitespaceBoundary(value) {
		return false
	}
	for _, r := range value {
		if r <= 0x1f || r >= 0x7f && r <= 0x9f {
			return false
		}
	}
	return true
}

func hasPortableWhitespaceBoundary(value string) bool {
	first, _ := utf8.DecodeRuneInString(value)
	last, _ := utf8.DecodeLastRuneInString(value)
	return isPortableWhitespace(first) || isPortableWhitespace(last)
}

func isPortableWhitespace(r rune) bool {
	switch {
	case r >= 0x09 && r <= 0x0d:
		return true
	case r == 0x20, r == 0x85, r == 0xa0, r == 0x1680, r == 0x2028, r == 0x2029,
		r == 0x202f, r == 0x205f, r == 0x3000:
		return true
	case r >= 0x2000 && r <= 0x200a:
		return true
	default:
		return false
	}
}

func validateContactEmail(fl validator.FieldLevel) bool {
	return ContactEmail(fl.Field().String())
}

// ContactEmail validates an already-normalized portable contact address.
func ContactEmail(value string) bool {
	if value == "" || len(value) > 254 || !isASCII(value) || strings.Count(value, "@") != 1 {
		return false
	}
	local, domain, _ := strings.Cut(value, "@")
	if len(local) < 1 || len(local) > 64 || local[0] == '.' || local[len(local)-1] == '.' ||
		strings.Contains(local, "..") {
		return false
	}
	for i := range len(local) {
		if !isEmailLocalCharacter(local[i]) {
			return false
		}
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return false
	}
	for _, label := range labels {
		if len(label) < 1 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := range len(label) {
			if !isASCIILetterOrDigit(label[i]) && label[i] != '-' {
				return false
			}
		}
	}
	return true
}

func validatePhoneNumber(fl validator.FieldLevel) bool {
	return PhoneNumber(fl.Field().String())
}

// PhoneNumber validates the exact portable E.164 subset.
func PhoneNumber(value string) bool {
	if len(value) < 8 || len(value) > 16 || value[0] != '+' || value[1] < '1' || value[1] > '9' {
		return false
	}
	for i := 2; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

// NormalizeContactEmail applies only the contract-authorized ASCII trimming
// and domain case-folding.
func NormalizeContactEmail(value string) string {
	value = StripASCIIWhitespace(value)
	local, domain, found := strings.Cut(value, "@")
	if !found {
		return value
	}
	return local + "@" + strings.ToLower(domain)
}

// StripASCIIWhitespace removes exactly U+0009..U+000D and U+0020 at the
// input boundary.
func StripASCIIWhitespace(value string) string {
	return strings.Trim(value, "\t\n\v\f\r ")
}

func isASCII(value string) bool {
	for i := range len(value) {
		if value[i] > 0x7f {
			return false
		}
	}
	return true
}

func isASCIILetterOrDigit(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}

func isEmailLocalCharacter(value byte) bool {
	if isASCIILetterOrDigit(value) {
		return true
	}
	return strings.ContainsRune("!#$%&'*+/=?^_{|}~.-", rune(value))
}
