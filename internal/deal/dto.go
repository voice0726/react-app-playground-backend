package deal

import (
	"errors"
	"github.com/google/uuid"
	"strings"
)

type CreateRequest struct {
	Title      string `json:"title"`
	CustomerID string `json:"customerId"`
	Amount     int64  `json:"amount"`
	Status     string `json:"status"`
}
type UpdateRequest struct {
	Title      *string `json:"title"`
	CustomerID *string `json:"customerId"`
	Amount     *int64  `json:"amount"`
	Status     *string `json:"status"`
}

func validStatus(s string) bool { return s == "open" || s == "won" || s == "lost" }
func (r CreateRequest) Validate() error {
	if strings.TrimSpace(r.Title) == "" || len(r.Title) > 200 || r.Amount < 0 || !validStatus(r.Status) {
		return errors.New("invalid deal")
	}
	if _, e := uuid.Parse(r.CustomerID); e != nil {
		return errors.New("invalid customerId")
	}
	return nil
}
func (r UpdateRequest) Validate() error {
	if r.Title != nil && (strings.TrimSpace(*r.Title) == "" || len(*r.Title) > 200) {
		return errors.New("invalid title")
	}
	if r.CustomerID != nil {
		if _, e := uuid.Parse(*r.CustomerID); e != nil {
			return errors.New("invalid customerId")
		}
	}
	if r.Amount != nil && *r.Amount < 0 {
		return errors.New("invalid amount")
	}
	if r.Status != nil && !validStatus(*r.Status) {
		return errors.New("invalid status")
	}
	return nil
}
