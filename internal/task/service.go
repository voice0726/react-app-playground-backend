package task

import (
	"context"
	"github.com/google/uuid"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
)

type Service struct{ repo Repository }

func NewService(r Repository) *Service                              { return &Service{r} }
func (s *Service) List(c context.Context, u string) ([]Task, error) { return s.repo.List(c, u) }
func (s *Service) Create(c context.Context, u string, r CreateRequest) (Task, error) {
	if e := r.Validate(); e != nil {
		return Task{}, httperror.New(httperror.Validation, "task_validation", "Invalid task.", e)
	}
	return s.repo.Create(c, u, Task{Title: r.Title, Description: r.Description, Status: r.Status})
}
func (s *Service) Update(c context.Context, u string, id uuid.UUID, r UpdateRequest) (Task, error) {
	if e := r.Validate(); e != nil {
		return Task{}, httperror.New(httperror.Validation, "task_validation", "Invalid task.", e)
	}
	return s.repo.Update(c, u, id, UpdatePatch{r.Title, r.Description, r.Status})
}
func (s *Service) Delete(c context.Context, u string, id uuid.UUID) error {
	return s.repo.Delete(c, u, id)
}
