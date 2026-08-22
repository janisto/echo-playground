package profile

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/fxamacker/cbor/v2"

	"github.com/janisto/echo-playground/internal/platform/validate"
)

// CreateInput is the closed GCP ProfileCreate request.
type CreateInput struct {
	FirstName      string       `json:"firstName"      cbor:"firstName"      validate:"required,bounded_name"  example:"Ada"`
	LastName       string       `json:"lastName"       cbor:"lastName"       validate:"required,bounded_name"  example:"Lovelace"`
	ContactEmail   string       `json:"contactEmail"   cbor:"contactEmail"   validate:"required,contact_email" example:"Ada@example.com"`
	PhoneNumber    string       `json:"phoneNumber"    cbor:"phoneNumber"    validate:"required,phone_number"  example:"+358401234567"`
	MarketingOptIn optionalBool `json:"marketingOptIn" cbor:"marketingOptIn"`
	TermsAccepted  bool         `json:"termsAccepted"  cbor:"termsAccepted"  validate:"required,eq=true"       example:"true"`
}

func (input *CreateInput) Normalize() {
	input.ContactEmail = validate.NormalizeContactEmail(input.ContactEmail)
	input.PhoneNumber = validate.StripASCIIWhitespace(input.PhoneNumber)
}

// UpdateInput is the closed, non-empty GCP ProfileUpdate request.
type UpdateInput struct {
	FirstName      optionalString `json:"firstName"      cbor:"firstName"`
	LastName       optionalString `json:"lastName"       cbor:"lastName"`
	ContactEmail   optionalString `json:"contactEmail"   cbor:"contactEmail"`
	PhoneNumber    optionalString `json:"phoneNumber"    cbor:"phoneNumber"`
	MarketingOptIn optionalBool   `json:"marketingOptIn" cbor:"marketingOptIn"`
}

func (input *UpdateInput) Normalize() {
	if input.ContactEmail.Present {
		input.ContactEmail.Value = validate.NormalizeContactEmail(input.ContactEmail.Value)
	}
	if input.PhoneNumber.Present {
		input.PhoneNumber.Value = validate.StripASCIIWhitespace(input.PhoneNumber.Value)
	}
}

func (input *UpdateInput) Empty() bool {
	return !input.FirstName.Present && !input.LastName.Present && !input.ContactEmail.Present &&
		!input.PhoneNumber.Present && !input.MarketingOptIn.Present
}

func (input *UpdateInput) ValidationTarget() updateValidation {
	return updateValidation{
		FirstName:      input.FirstName.Pointer(),
		LastName:       input.LastName.Pointer(),
		ContactEmail:   input.ContactEmail.Pointer(),
		PhoneNumber:    input.PhoneNumber.Pointer(),
		MarketingOptIn: input.MarketingOptIn.Pointer(),
	}
}

type updateValidation struct {
	FirstName      *string `json:"firstName"      validate:"omitempty,bounded_name"`
	LastName       *string `json:"lastName"       validate:"omitempty,bounded_name"`
	ContactEmail   *string `json:"contactEmail"   validate:"omitempty,contact_email"`
	PhoneNumber    *string `json:"phoneNumber"    validate:"omitempty,phone_number"`
	MarketingOptIn *bool   `json:"marketingOptIn"`
}

type optionalString struct {
	Value   string
	Present bool
}

func (value *optionalString) UnmarshalJSON(data []byte) error {
	value.Present = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return errors.New("null is not allowed")
	}
	return json.Unmarshal(data, &value.Value)
}

func (value *optionalString) UnmarshalCBOR(data []byte) error {
	value.Present = true
	if bytes.Equal(data, []byte{0xf6}) {
		return errors.New("null is not allowed")
	}
	return cbor.Unmarshal(data, &value.Value)
}

func (value *optionalString) Pointer() *string {
	if !value.Present {
		return nil
	}
	return &value.Value
}

type optionalBool struct {
	Value   bool
	Present bool
}

func (value *optionalBool) UnmarshalJSON(data []byte) error {
	value.Present = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return errors.New("null is not allowed")
	}
	return json.Unmarshal(data, &value.Value)
}

func (value *optionalBool) UnmarshalCBOR(data []byte) error {
	value.Present = true
	if bytes.Equal(data, []byte{0xf6}) {
		return errors.New("null is not allowed")
	}
	return cbor.Unmarshal(data, &value.Value)
}

func (value *optionalBool) Pointer() *bool {
	if !value.Present {
		return nil
	}
	return &value.Value
}
