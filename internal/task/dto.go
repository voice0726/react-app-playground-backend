package task

import (
	"errors"
	"strings"
)

type CreateRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
}
type UpdateRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
}

func valid(s string) bool { return s == "TODO" || s == "IN_PROGRESS" || s == "DONE" }
func (r CreateRequest) Validate() error {
	if strings.TrimSpace(r.Title) == "" || len(r.Title) > 200 || len(r.Description) > 2000 || !valid(r.Status) {
		return errors.New("invalid task")
	}
	return nil
}
func (r UpdateRequest) Validate() error {
	if r.Title != nil && (strings.TrimSpace(*r.Title) == "" || len(*r.Title) > 200) {
		return errors.New("invalid title")
	}
	if r.Description != nil && len(*r.Description) > 2000 {
		return errors.New("invalid description")
	}
	if r.Status != nil && !valid(*r.Status) {
		return errors.New("invalid status")
	}
	return nil
}
