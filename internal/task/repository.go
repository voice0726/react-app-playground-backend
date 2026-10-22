package task

import (
	"context"

	"github.com/google/uuid"
	"github.com/voice0726/react-app-playground-backend/internal/shared/pagination"
)

type UpdatePatch struct {
	Title       *string
	Description *string
	Status      *string
}
type ListOptions struct {
	Query  pagination.Query
	Status string
}
type Repository interface {
	List(context.Context, uuid.UUID, ListOptions) ([]Task, int64, error)
	Create(context.Context, uuid.UUID, string, Task) (Task, error)
	Update(context.Context, uuid.UUID, uuid.UUID, UpdatePatch) (Task, error)
	Delete(context.Context, uuid.UUID, uuid.UUID) error
}
