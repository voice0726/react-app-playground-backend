package deal

import (
	"context"

	"github.com/google/uuid"
	"github.com/voice0726/react-app-playground-backend/internal/customer"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/orgcontext"
)

type Service struct {
	repo   Repository
	finder customer.CustomerFinder
}

func NewService(r Repository, f customer.CustomerFinder) *Service { return &Service{r, f} }
func (s *Service) List(c context.Context, scope orgcontext.Scope, options ListOptions) ([]Deal, int64, error) {
	return s.repo.List(c, scope.OrganizationID, options)
}
func (s *Service) Find(c context.Context, scope orgcontext.Scope, id uuid.UUID) (Deal, error) {
	return s.repo.Find(c, scope.OrganizationID, id)
}
func (s *Service) Create(c context.Context, scope orgcontext.Scope, r CreateRequest) (Deal, error) {
	if e := r.Validate(); e != nil {
		return Deal{}, httperror.New(httperror.Validation, "deal_validation", "Invalid deal.", e)
	}
	cid, _ := uuid.Parse(r.CustomerID)
	if _, e := s.finder.FindByOrganization(c, scope.OrganizationID, cid); e != nil {
		return Deal{}, e
	}
	return s.repo.Create(c, scope.OrganizationID, scope.UserID, Deal{CustomerID: cid, Title: r.Title, Amount: r.Amount, Status: r.Status})
}
func (s *Service) Update(c context.Context, scope orgcontext.Scope, id uuid.UUID, r UpdateRequest) (Deal, error) {
	if e := r.Validate(); e != nil {
		return Deal{}, httperror.New(httperror.Validation, "deal_validation", "Invalid deal.", e)
	}
	var cid *uuid.UUID
	if r.CustomerID != nil {
		x, _ := uuid.Parse(*r.CustomerID)
		cid = &x
		if _, e := s.finder.FindByOrganization(c, scope.OrganizationID, x); e != nil {
			return Deal{}, e
		}
	}
	return s.repo.Update(c, scope.OrganizationID, id, UpdatePatch{r.Title, cid, r.Amount, r.Status})
}
