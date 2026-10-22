package task

import (
	"context"

	"github.com/google/uuid"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/orgcontext"
)

type Service struct{ repo Repository }

func NewService(r Repository) *Service { return &Service{r} }
func (s *Service) List(c context.Context, scope orgcontext.Scope, options ListOptions) ([]Task, int64, error) {
	return s.repo.List(c, scope.OrganizationID, options)
}
func (s *Service) Create(c context.Context, scope orgcontext.Scope, r CreateRequest) (Task, error) {
	if e := r.Validate(); e != nil {
		return Task{}, httperror.New(httperror.Validation, "task_validation", "Invalid task.", e)
	}
	return s.repo.Create(c, scope.OrganizationID, scope.UserID, Task{Title: r.Title, Description: r.Description, Status: r.Status})
}
func (s *Service) Update(c context.Context, scope orgcontext.Scope, id uuid.UUID, r UpdateRequest) (Task, error) {
	if e := r.Validate(); e != nil {
		return Task{}, httperror.New(httperror.Validation, "task_validation", "Invalid task.", e)
	}
	return s.repo.Update(c, scope.OrganizationID, id, UpdatePatch{r.Title, r.Description, r.Status})
}
func (s *Service) Delete(c context.Context, scope orgcontext.Scope, id uuid.UUID) error {
	return s.repo.Delete(c, scope.OrganizationID, id)
}
