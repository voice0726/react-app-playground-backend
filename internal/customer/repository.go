package customer

import (
	"context"

	"github.com/google/uuid"
	"github.com/voice0726/react-app-playground-backend/internal/shared/pagination"
)

type UpdatePatch struct {
	Name   *string
	Email  *string
	Phone  *string
	Status *string
}
type ListOptions struct {
	Query  pagination.Query
	Status string
}
type Repository interface {
	List(context.Context, uuid.UUID, ListOptions) ([]Customer, int64, error)
	Find(context.Context, uuid.UUID, uuid.UUID) (Customer, error)
	Create(context.Context, uuid.UUID, string, Customer) (Customer, error)
	Update(context.Context, uuid.UUID, uuid.UUID, UpdatePatch) (Customer, error)
	Delete(context.Context, uuid.UUID, uuid.UUID) error
}
type CustomerFinder interface {
	FindByOrganization(context.Context, uuid.UUID, uuid.UUID) (Customer, error)
}
