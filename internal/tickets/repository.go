package tickets

import "context"

type Repository interface {
	List(ctx context.Context, userID string) ([]Ticket, error)
	Find(ctx context.Context, userID string, id uint64) (Ticket, error)
	Create(ctx context.Context, userID string, ticket Ticket) (Ticket, error)
	Replace(ctx context.Context, userID string, ticket Ticket, history []HistoryItem) (Ticket, error)
	AddComment(ctx context.Context, userID string, ticket Ticket, comment Comment) (Ticket, error)
	UpdateComment(ctx context.Context, userID string, ticket Ticket, comment Comment) (Ticket, error)
	DeleteComment(ctx context.Context, userID string, ticket Ticket, commentID uint64) (Ticket, error)
	Delete(ctx context.Context, userID string, id uint64) error
}
