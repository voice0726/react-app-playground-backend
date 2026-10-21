package tickets

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	List(ctx context.Context, organizationID uuid.UUID) ([]Ticket, error)
	Find(ctx context.Context, organizationID uuid.UUID, id uint64) (Ticket, error)
	Create(ctx context.Context, organizationID uuid.UUID, ticket Ticket) (Ticket, error)
	UpdateLocked(ctx context.Context, organizationID uuid.UUID, id uint64, apply func(Ticket) (Ticket, []HistoryItem, error)) (Ticket, error)
	AddComment(ctx context.Context, organizationID uuid.UUID, id uint64, updatedBy Actor, now func() time.Time, comment Comment) (Ticket, error)
	UpdateComment(ctx context.Context, organizationID uuid.UUID, id, commentID uint64, actor Actor, now func() time.Time, body string) (Ticket, error)
	DeleteComment(ctx context.Context, organizationID uuid.UUID, id, commentID uint64, actor Actor, now func() time.Time) (Ticket, error)
	Delete(ctx context.Context, organizationID uuid.UUID, id uint64) error
}
