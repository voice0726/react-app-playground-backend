package customer

import (
	"context"

	"github.com/google/uuid"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/orgcontext"
)

type Service struct{ repo Repository }

func NewService(repo Repository) *Service { return &Service{repo} }
func (s *Service) List(c context.Context, scope orgcontext.Scope, options ListOptions) ([]Customer, int64, error) {
	return s.repo.List(c, scope.OrganizationID, options)
}
func (s *Service) Find(c context.Context, scope orgcontext.Scope, id uuid.UUID) (Customer, error) {
	return s.repo.Find(c, scope.OrganizationID, id)
}
func (s *Service) FindByOrganization(c context.Context, organizationID, id uuid.UUID) (Customer, error) {
	return s.repo.Find(c, organizationID, id)
}
func (s *Service) Create(c context.Context, scope orgcontext.Scope, r CreateRequest) (Customer, error) {
	if err := r.Validate(); err != nil {
		return Customer{}, httperror.New(httperror.Validation, "customer_validation", "Invalid customer.", err)
	}
	return s.repo.Create(c, scope.OrganizationID, scope.UserID, Customer{Name: r.Name, Email: r.Email, Phone: r.Phone, Status: r.Status})
}
func (s *Service) Update(c context.Context, scope orgcontext.Scope, id uuid.UUID, r UpdateRequest) (Customer, error) {
	if err := r.Validate(); err != nil {
		return Customer{}, httperror.New(httperror.Validation, "customer_validation", "Invalid customer.", err)
	}
	return s.repo.Update(c, scope.OrganizationID, id, UpdatePatch{r.Name, r.Email, r.Phone, r.Status})
}
func (s *Service) Delete(c context.Context, scope orgcontext.Scope, id uuid.UUID) error {
	return s.repo.Delete(c, scope.OrganizationID, id)
}
