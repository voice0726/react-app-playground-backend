package invoice

import (
	"errors"
	"github.com/google/uuid"
	"strings"
	"time"
)

type CreateRequest struct {
	Number     string `json:"number"`
	CustomerID string `json:"customerId"`
	Amount     int64  `json:"amount"`
	IssueDate  string `json:"issueDate"`
	DueDate    string `json:"dueDate"`
	Status     string `json:"status"`
}
type UpdateRequest struct {
	Number     *string `json:"number"`
	CustomerID *string `json:"customerId"`
	Amount     *int64  `json:"amount"`
	IssueDate  *string `json:"issueDate"`
	DueDate    *string `json:"dueDate"`
	Status     *string `json:"status"`
}

func status(s string) bool             { return s == "draft" || s == "sent" || s == "paid" || s == "cancelled" }
func date(s string) (time.Time, error) { return time.Parse("2006-01-02", s) }
func (r CreateRequest) Validate() error {
	if strings.TrimSpace(r.Number) == "" || len(r.Number) > 100 || r.Amount < 0 || !status(r.Status) {
		return errors.New("invalid invoice")
	}
	if _, e := uuid.Parse(r.CustomerID); e != nil {
		return errors.New("invalid customer")
	}
	a, e := date(r.IssueDate)
	if e != nil {
		return errors.New("invalid issueDate")
	}
	b, e := date(r.DueDate)
	if e != nil || b.Before(a) {
		return errors.New("invalid dueDate")
	}
	return nil
}
func (r UpdateRequest) Validate() error {
	if r.Number != nil && (strings.TrimSpace(*r.Number) == "" || len(*r.Number) > 100) {
		return errors.New("invalid number")
	}
	if r.CustomerID != nil {
		if _, e := uuid.Parse(*r.CustomerID); e != nil {
			return errors.New("invalid customer")
		}
	}
	if r.Amount != nil && *r.Amount < 0 {
		return errors.New("invalid amount")
	}
	if r.IssueDate != nil {
		if _, e := date(*r.IssueDate); e != nil {
			return errors.New("invalid issueDate")
		}
	}
	if r.DueDate != nil {
		if _, e := date(*r.DueDate); e != nil {
			return errors.New("invalid dueDate")
		}
	}
	if r.Status != nil && !status(*r.Status) {
		return errors.New("invalid status")
	}
	return nil
}
