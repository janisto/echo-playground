package validate

import (
	"errors"
	"testing"
)

type createInput struct {
	Name  string `json:"name"         validate:"required,min=1,max=100"`
	Email string `json:"email"        validate:"required,email"`
	Phone string `json:"phone_number" validate:"required,e164"`
}

type listInput struct {
	Cursor   string `query:"cursor"`
	Limit    int    `query:"limit"    validate:"omitempty,min=1,max=100"`
	Category string `query:"category" validate:"omitempty,oneof=electronics tools accessories"`
}

type pathInput struct {
	ID string `param:"id" validate:"required"`
}

type mixedInput struct {
	ID   string `param:"id" validate:"required"`
	Name string `           validate:"required,min=1,max=100" json:"name"`
}

func TestValidate_ValidInput(t *testing.T) {
	v := New()
	input := createInput{
		Name:  "Alice",
		Email: "alice@example.com",
		Phone: "+1234567890",
	}
	if err := v.Validate(input); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidate_RequiredFields(t *testing.T) {
	v := New()
	input := createInput{}
	err := v.Validate(input)
	if err == nil {
		t.Fatal("expected validation error")
	}

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if ve.Message != "validation failed" {
		t.Fatalf("expected 'validation failed', got %q", ve.Message)
	}
	if len(ve.Fields) != 3 {
		t.Fatalf("expected 3 field errors, got %d", len(ve.Fields))
	}

	fieldMap := make(map[string]FieldError)
	for _, f := range ve.Fields {
		fieldMap[f.Field] = f
	}

	assertField(t, fieldMap, "name", "name is required")
	assertField(t, fieldMap, "email", "email is required")
	assertField(t, fieldMap, "phone_number", "phone_number is required")
}

func TestValidate_InvalidEmail(t *testing.T) {
	v := New()
	input := createInput{
		Name:  "Alice",
		Email: "not-an-email",
		Phone: "+1234567890",
	}
	err := v.Validate(input)
	if err == nil {
		t.Fatal("expected validation error")
	}

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if len(ve.Fields) != 1 {
		t.Fatalf("expected 1 field error, got %d", len(ve.Fields))
	}
	if ve.Fields[0].Field != "email" {
		t.Fatalf("expected field 'email', got %q", ve.Fields[0].Field)
	}
	if ve.Fields[0].Message != "email must be a valid email address" {
		t.Fatalf("unexpected message: %s", ve.Fields[0].Message)
	}
	if ve.Fields[0].Value != "not-an-email" {
		t.Fatalf("expected value 'not-an-email', got %q", ve.Fields[0].Value)
	}
}

func TestValidate_InvalidE164(t *testing.T) {
	v := New()
	input := createInput{
		Name:  "Alice",
		Email: "alice@example.com",
		Phone: "12345",
	}
	err := v.Validate(input)
	if err == nil {
		t.Fatal("expected validation error")
	}

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if len(ve.Fields) != 1 {
		t.Fatalf("expected 1 field error, got %d", len(ve.Fields))
	}
	if ve.Fields[0].Field != "phone_number" {
		t.Fatalf("expected field 'phone_number', got %q", ve.Fields[0].Field)
	}
}

func TestValidate_MinMax(t *testing.T) {
	v := New()
	input := listInput{Limit: 0}
	if err := v.Validate(input); err != nil {
		t.Fatal("limit=0 with omitempty should pass")
	}

	input = listInput{Limit: 101}
	err := v.Validate(input)
	if err == nil {
		t.Fatal("expected validation error for limit=101")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if len(ve.Fields) != 1 {
		t.Fatalf("expected 1 field error, got %d", len(ve.Fields))
	}
	if ve.Fields[0].Field != "limit" {
		t.Fatalf("expected field 'limit', got %q", ve.Fields[0].Field)
	}
	if ve.Fields[0].Message != "limit must be at most 100" {
		t.Fatalf("unexpected message: %s", ve.Fields[0].Message)
	}
}

func TestValidate_MinNegative(t *testing.T) {
	v := New()
	input := listInput{Limit: -1}
	err := v.Validate(input)
	if err == nil {
		t.Fatal("expected validation error for limit=-1")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if ve.Fields[0].Field != "limit" {
		t.Fatalf("expected field 'limit', got %q", ve.Fields[0].Field)
	}
	if ve.Fields[0].Message != "limit must be at least 1" {
		t.Fatalf("unexpected message: %s", ve.Fields[0].Message)
	}
}

func TestValidate_Oneof(t *testing.T) {
	v := New()
	input := listInput{Category: "electronics"}
	if err := v.Validate(input); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	input = listInput{Category: "invalid"}
	err := v.Validate(input)
	if err == nil {
		t.Fatal("expected validation error for invalid category")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if ve.Fields[0].Field != "category" {
		t.Fatalf("expected field 'category', got %q", ve.Fields[0].Field)
	}
	if ve.Fields[0].Message != "category must be one of: electronics tools accessories" {
		t.Fatalf("unexpected message: %s", ve.Fields[0].Message)
	}
}

func TestValidate_QueryTagNames(t *testing.T) {
	v := New()
	input := listInput{Limit: -1}
	err := v.Validate(input)
	if err == nil {
		t.Fatal("expected validation error")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if ve.Fields[0].Field != "limit" {
		t.Fatalf("expected query tag name 'limit', got %q", ve.Fields[0].Field)
	}
}

func TestValidate_ParamTagNames(t *testing.T) {
	v := New()
	input := pathInput{}
	err := v.Validate(input)
	if err == nil {
		t.Fatal("expected validation error")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if ve.Fields[0].Field != "id" {
		t.Fatalf("expected param tag name 'id', got %q", ve.Fields[0].Field)
	}
}

func TestValidate_MixedTags(t *testing.T) {
	v := New()
	input := mixedInput{}
	err := v.Validate(input)
	if err == nil {
		t.Fatal("expected validation error")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if len(ve.Fields) != 2 {
		t.Fatalf("expected 2 field errors, got %d", len(ve.Fields))
	}

	fieldMap := make(map[string]FieldError)
	for _, f := range ve.Fields {
		fieldMap[f.Field] = f
	}
	assertField(t, fieldMap, "id", "id is required")
	assertField(t, fieldMap, "name", "name is required")
}

func TestValidationError_ErrorMethod(t *testing.T) {
	ve := &ValidationError{Message: "validation failed"}
	if ve.Error() != "validation failed" {
		t.Fatalf("expected 'validation failed', got %q", ve.Error())
	}
}

func TestValidate_StringMinMax(t *testing.T) {
	v := New()
	input := createInput{
		Name:  "A",
		Email: "a@b.com",
		Phone: "+1234567890",
	}
	if err := v.Validate(input); err != nil {
		t.Fatalf("expected no error for name length 1, got %v", err)
	}
}

func assertField(t *testing.T, fields map[string]FieldError, name, expectedMsg string) {
	t.Helper()
	fe, ok := fields[name]
	if !ok {
		t.Fatalf("missing field error for %q", name)
	}
	if fe.Message != expectedMsg {
		t.Fatalf("field %q: expected message %q, got %q", name, expectedMsg, fe.Message)
	}
}
