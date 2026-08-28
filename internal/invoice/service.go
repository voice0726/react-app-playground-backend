package invoice

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/voice0726/react-app-playground-backend/internal/customer"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
)

type Service struct {
	repo   Repository
	finder customer.CustomerFinder
}

func NewService(r Repository, f customer.CustomerFinder) *Service      { return &Service{r, f} }
func (s *Service) List(c context.Context, u string) ([]Invoice, error) { return s.repo.List(c, u) }
func (s *Service) Find(c context.Context, u string, id uuid.UUID) (Invoice, error) {
	return s.repo.Find(c, u, id)
}
func (s *Service) Create(c context.Context, u string, r CreateRequest) (Invoice, error) {
	if e := r.Validate(); e != nil {
		return Invoice{}, httperror.New(httperror.Validation, "invoice_validation", "Invalid invoice.", e)
	}
	cid, _ := uuid.Parse(r.CustomerID)
	if _, e := s.finder.Find(c, u, cid); e != nil {
		return Invoice{}, e
	}
	a, _ := date(r.IssueDate)
	b, _ := date(r.DueDate)
	return s.repo.Create(c, u, Invoice{CustomerID: cid, Number: r.Number, Amount: r.Amount, IssueDate: a, DueDate: b, Status: r.Status})
}
func (s *Service) Update(c context.Context, u string, id uuid.UUID, r UpdateRequest) (Invoice, error) {
	if e := r.Validate(); e != nil {
		return Invoice{}, httperror.New(httperror.Validation, "invoice_validation", "Invalid invoice.", e)
	}
	return s.repo.UpdateLocked(c, u, id, func(cur Invoice) (Invoice, error) {
		if r.Status != nil {
			if e := CanTransition(cur.Status, *r.Status); e != nil {
				return cur, e
			}
			cur.Status = *r.Status
		}
		if r.Number != nil {
			cur.Number = *r.Number
		}
		if r.Amount != nil {
			cur.Amount = *r.Amount
		}
		if r.CustomerID != nil {
			x, _ := uuid.Parse(*r.CustomerID)
			if _, e := s.finder.Find(c, u, x); e != nil {
				return cur, e
			}
			cur.CustomerID = x
		}
		if r.IssueDate != nil {
			x, _ := date(*r.IssueDate)
			cur.IssueDate = x
		}
		if r.DueDate != nil {
			x, _ := date(*r.DueDate)
			cur.DueDate = x
		}
		if cur.DueDate.Before(cur.IssueDate) {
			return cur, httperror.New(httperror.Validation, "invoice_validation", "Invalid invoice dates.", errors.New("due date must not be before issue date"))
		}
		return cur, nil
	})
}
