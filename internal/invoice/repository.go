package invoice

import (
	"context"
	"github.com/google/uuid"
	"time"
)

type UpdatePatch struct {
	Number     *string
	CustomerID *uuid.UUID
	Amount     *int64
	IssueDate  *time.Time
	DueDate    *time.Time
	Status     *string
}
type Repository interface {
	List(context.Context, string) ([]Invoice, error)
	Find(context.Context, string, uuid.UUID) (Invoice, error)
	Create(context.Context, string, Invoice) (Invoice, error)
	UpdateLocked(context.Context, string, uuid.UUID, func(Invoice) (Invoice, error)) (Invoice, error)
}
