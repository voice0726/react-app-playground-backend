package task

import (
	"context"
	"github.com/google/uuid"
)

type UpdatePatch struct {
	Title       *string
	Description *string
	Status      *string
}
type Repository interface {
	List(context.Context, string) ([]Task, error)
	Create(context.Context, string, Task) (Task, error)
	Update(context.Context, string, uuid.UUID, UpdatePatch) (Task, error)
	Delete(context.Context, string, uuid.UUID) error
}
