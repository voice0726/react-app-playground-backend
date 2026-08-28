package customer

import (
	"context"
	"github.com/google/uuid"
)

type UpdatePatch struct {
	Name   *string
	Email  *string
	Phone  *string
	Status *string
}
type Repository interface {
	List(context.Context, string) ([]Customer, error)
	Find(context.Context, string, uuid.UUID) (Customer, error)
	Create(context.Context, string, Customer) (Customer, error)
	Update(context.Context, string, uuid.UUID, UpdatePatch) (Customer, error)
	Delete(context.Context, string, uuid.UUID) error
}
type CustomerFinder interface {
	Find(context.Context, string, uuid.UUID) (Customer, error)
}
