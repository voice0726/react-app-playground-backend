package customer

import (
	"context"
	"github.com/google/uuid"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
)

type Service struct{ repo Repository }

func NewService(repo Repository) *Service                               { return &Service{repo} }
func (s *Service) List(c context.Context, u string) ([]Customer, error) { return s.repo.List(c, u) }
func (s *Service) Find(c context.Context, u string, id uuid.UUID) (Customer, error) {
	return s.repo.Find(c, u, id)
}
func (s *Service) Create(c context.Context, u string, r CreateRequest) (Customer, error) {
	if err := r.Validate(); err != nil {
		return Customer{}, httperror.New(httperror.Validation, "customer_validation", "Invalid customer.", err)
	}
	return s.repo.Create(c, u, Customer{Name: r.Name, Email: r.Email, Phone: r.Phone, Status: r.Status})
}
func (s *Service) Update(c context.Context, u string, id uuid.UUID, r UpdateRequest) (Customer, error) {
	if err := r.Validate(); err != nil {
		return Customer{}, httperror.New(httperror.Validation, "customer_validation", "Invalid customer.", err)
	}
	return s.repo.Update(c, u, id, UpdatePatch{r.Name, r.Email, r.Phone, r.Status})
}
func (s *Service) Delete(c context.Context, u string, id uuid.UUID) error {
	return s.repo.Delete(c, u, id)
}
