package tickets

import (
	"context"
	"time"
)

type Repository interface {
	List(ctx context.Context, userID string) ([]Ticket, error)
	Find(ctx context.Context, userID string, id uint64) (Ticket, error)
	Create(ctx context.Context, userID string, ticket Ticket) (Ticket, error)
	UpdateLocked(ctx context.Context, userID string, id uint64, apply func(Ticket) (Ticket, []HistoryItem, error)) (Ticket, error)
	AddComment(ctx context.Context, userID string, id uint64, updatedBy Actor, now func() time.Time, comment Comment) (Ticket, error)
	UpdateComment(ctx context.Context, userID string, id, commentID uint64, actor Actor, now func() time.Time, body string) (Ticket, error)
	DeleteComment(ctx context.Context, userID string, id, commentID uint64, actor Actor, now func() time.Time) (Ticket, error)
	Delete(ctx context.Context, userID string, id uint64) error
}
