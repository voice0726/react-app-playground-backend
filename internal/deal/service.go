package deal

import (
	"context"
	"github.com/google/uuid"
	"github.com/voice0726/react-app-playground-backend/internal/customer"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
)

type Service struct {
	repo   Repository
	finder customer.CustomerFinder
}

func NewService(r Repository, f customer.CustomerFinder) *Service   { return &Service{r, f} }
func (s *Service) List(c context.Context, u string) ([]Deal, error) { return s.repo.List(c, u) }
func (s *Service) Find(c context.Context, u string, id uuid.UUID) (Deal, error) {
	return s.repo.Find(c, u, id)
}
func (s *Service) Create(c context.Context, u string, r CreateRequest) (Deal, error) {
	if e := r.Validate(); e != nil {
		return Deal{}, httperror.New(httperror.Validation, "deal_validation", "Invalid deal.", e)
	}
	cid, _ := uuid.Parse(r.CustomerID)
	if _, e := s.finder.Find(c, u, cid); e != nil {
		return Deal{}, e
	}
	return s.repo.Create(c, u, Deal{CustomerID: cid, Title: r.Title, Amount: r.Amount, Status: r.Status})
}
func (s *Service) Update(c context.Context, u string, id uuid.UUID, r UpdateRequest) (Deal, error) {
	if e := r.Validate(); e != nil {
		return Deal{}, httperror.New(httperror.Validation, "deal_validation", "Invalid deal.", e)
	}
	var cid *uuid.UUID
	if r.CustomerID != nil {
		x, _ := uuid.Parse(*r.CustomerID)
		cid = &x
		if _, e := s.finder.Find(c, u, x); e != nil {
			return Deal{}, e
		}
	}
	return s.repo.Update(c, u, id, UpdatePatch{r.Title, cid, r.Amount, r.Status})
}
