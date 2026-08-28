package deal

import (
	"context"
	"github.com/google/uuid"
)

type UpdatePatch struct {
	Title      *string
	CustomerID *uuid.UUID
	Amount     *int64
	Status     *string
}
type Repository interface {
	List(context.Context, string) ([]Deal, error)
	Find(context.Context, string, uuid.UUID) (Deal, error)
	Create(context.Context, string, Deal) (Deal, error)
	Update(context.Context, string, uuid.UUID, UpdatePatch) (Deal, error)
}
