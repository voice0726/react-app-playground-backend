package organization

import (
	"errors"
	"strings"
	"unicode/utf8"
)

const (
	maxOrganizationName = 200
	maxMemberUserID     = 200
)

type CreateRequest struct {
	Name string `json:"name"`
}

type MemberRequest struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
}

func validateName(name string) error {
	if strings.TrimSpace(name) == "" || utf8.RuneCountInString(name) > maxOrganizationName {
		return errors.New("invalid organization name")
	}
	return nil
}

func (r CreateRequest) Validate() error { return validateName(r.Name) }

func (r MemberRequest) Validate() error {
	if strings.TrimSpace(r.UserID) == "" || utf8.RuneCountInString(r.UserID) > maxMemberUserID {
		return errors.New("invalid member userId")
	}
	if !ValidRole(r.Role) {
		return errors.New("invalid member role")
	}
	return nil
}
