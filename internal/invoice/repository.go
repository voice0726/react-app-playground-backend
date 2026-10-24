package invoice

import (
	"context"

	"github.com/google/uuid"
	"github.com/voice0726/react-app-playground-backend/internal/shared/pagination"
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
type ListOptions struct {
	Query      pagination.Query
	Status     string
	CustomerID *uuid.UUID
	Overdue    bool
	Today      time.Time
}
type Repository interface {
	List(context.Context, uuid.UUID, ListOptions) ([]Invoice, int64, int64, error)
	Find(context.Context, uuid.UUID, uuid.UUID) (Invoice, error)
	Create(context.Context, uuid.UUID, string, Invoice) (Invoice, error)
	UpdateLocked(context.Context, uuid.UUID, uuid.UUID, func(Invoice) (Invoice, error)) (Invoice, error)
	UpdateStatuses(context.Context, uuid.UUID, []uuid.UUID, string, string) ([]Invoice, error)
	Summary(context.Context, uuid.UUID, time.Time, *uuid.UUID) (Summary, error)
}
