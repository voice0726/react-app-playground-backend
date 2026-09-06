package deal

import (
	"context"

	"github.com/google/uuid"
	"github.com/voice0726/react-app-playground-backend/internal/shared/pagination"
)

type UpdatePatch struct {
	Title      *string
	CustomerID *uuid.UUID
	Amount     *int64
	Status     *string
}
type ListOptions struct {
	Query      pagination.Query
	Status     string
	CustomerID *uuid.UUID
}
type Repository interface {
	List(context.Context, uuid.UUID, ListOptions) ([]Deal, int64, error)
	Find(context.Context, uuid.UUID, uuid.UUID) (Deal, error)
	Create(context.Context, uuid.UUID, string, Deal) (Deal, error)
	Update(context.Context, uuid.UUID, uuid.UUID, UpdatePatch) (Deal, error)
}
