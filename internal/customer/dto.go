package customer

import (
	"errors"
	"net/mail"
	"strings"
)

type CreateRequest struct {
	Name   string `json:"name"`
	Email  string `json:"email"`
	Phone  string `json:"phone"`
	Status string `json:"status"`
}
type UpdateRequest struct {
	Name   *string `json:"name"`
	Email  *string `json:"email"`
	Phone  *string `json:"phone"`
	Status *string `json:"status"`
}

func validate(name, email, phone, status string) error {
	if name == "" || len(name) > 200 || email == "" || len(email) > 320 || phone == "" || len(phone) > 50 {
		return errors.New("invalid customer fields")
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		return errors.New("invalid email")
	}
	if status != "active" && status != "inactive" {
		return errors.New("invalid status")
	}
	return nil
}
func (r CreateRequest) Validate() error {
	return validate(strings.TrimSpace(r.Name), r.Email, strings.TrimSpace(r.Phone), r.Status)
}
func (r UpdateRequest) Validate() error {
	if r.Name != nil && (strings.TrimSpace(*r.Name) == "" || len(*r.Name) > 200) {
		return errors.New("invalid name")
	}
	if r.Email != nil {
		parsed, err := mail.ParseAddress(*r.Email)
		if err != nil || parsed.Address != *r.Email || len(*r.Email) > 320 {
			return errors.New("invalid email")
		}
	}
	if r.Phone != nil && (strings.TrimSpace(*r.Phone) == "" || len(*r.Phone) > 50) {
		return errors.New("invalid phone")
	}
	if r.Status != nil && *r.Status != "active" && *r.Status != "inactive" {
		return errors.New("invalid status")
	}
	return nil
}
