package tickets

import (
	"context"
	"errors"
	"time"

	"github.com/voice0726/react-app-playground-backend/internal/shared/dberror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GORMRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GORMRepository { return &GORMRepository{db} }

func (r *GORMRepository) List(ctx context.Context, user string) ([]Ticket, error) {
	var out []Ticket
	err := r.db.WithContext(ctx).Where("owner_user_id = ?", user).Order("created_at ASC").Order("id ASC").Find(&out).Error
	if err != nil {
		return nil, dberror.Classify(err)
	}
	return out, nil
}

func (r *GORMRepository) Find(ctx context.Context, user string, id uint64) (Ticket, error) {
	out, err := find(r.db.WithContext(ctx), user, id)
	if err != nil {
		return Ticket{}, classify(err)
	}
	return out, nil
}

func (r *GORMRepository) Create(ctx context.Context, user string, ticket Ticket) (Ticket, error) {
	ticket.OwnerUserID = user
	ticket.CreatedAt = ticket.CreatedAt.UTC()
	ticket.UpdatedAt = ticket.UpdatedAt.UTC()
	if err := r.db.WithContext(ctx).Create(&ticket).Error; err != nil {
		return Ticket{}, dberror.Classify(err)
	}
	ticket.History = []HistoryItem{}
	ticket.Comments = []Comment{}
	return ticket, nil
}

func (r *GORMRepository) UpdateLocked(ctx context.Context, user string, id uint64, apply func(Ticket) (Ticket, []HistoryItem, error)) (Ticket, error) {
	var out Ticket
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := lockTicket(tx, user, id)
		if err != nil {
			return err
		}
		next, history, err := apply(current)
		if err != nil {
			return err
		}
		next.ID, next.OwnerUserID = id, user
		result := tx.Model(&Ticket{}).Where("owner_user_id = ? AND id = ?", user, id).Updates(map[string]any{
			"title": next.Title, "status": next.Status, "assignee": next.Assignee,
			"updated_by_id": next.UpdatedBy.ID, "updated_by_name": next.UpdatedBy.Name,
			"updated_by_email": next.UpdatedBy.Email, "updated_at": next.UpdatedAt.UTC(),
		})
		if result.Error != nil {
			return result.Error
		}
		if records := toHistoryRecords(user, id, history); len(records) > 0 {
			if err := tx.Create(&records).Error; err != nil {
				return err
			}
		}
		found, err := find(tx, user, id)
		out = found
		return err
	})
	if err != nil {
		return Ticket{}, classify(err)
	}
	return out, nil
}

func (r *GORMRepository) AddComment(ctx context.Context, user string, id uint64, updatedBy Actor, now func() time.Time, comment Comment) (Ticket, error) {
	return r.mutateComment(ctx, user, id, updatedBy, now, func(tx *gorm.DB, changedAt time.Time) error {
		comment.OwnerUserID = user
		comment.TicketID = id
		comment.CreatedAt = changedAt
		return tx.Create(&comment).Error
	})
}

func (r *GORMRepository) UpdateComment(ctx context.Context, user string, id, commentID uint64, actor Actor, now func() time.Time, body string) (Ticket, error) {
	return r.mutateComment(ctx, user, id, actor, now, func(tx *gorm.DB, _ time.Time) error {
		comment, err := findComment(tx, user, id, commentID)
		if err != nil {
			return err
		}
		if comment.CreatedBy.ID != actor.ID {
			return ErrCommentPermissionDenied
		}
		result := tx.Model(&Comment{}).
			Where("owner_user_id = ? AND ticket_id = ? AND id = ?", user, id, commentID).
			Update("body", body)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrCommentNotFound
		}
		return nil
	})
}

func (r *GORMRepository) DeleteComment(ctx context.Context, user string, id, commentID uint64, actor Actor, now func() time.Time) (Ticket, error) {
	return r.mutateComment(ctx, user, id, actor, now, func(tx *gorm.DB, _ time.Time) error {
		comment, err := findComment(tx, user, id, commentID)
		if err != nil {
			return err
		}
		if comment.CreatedBy.ID != actor.ID {
			return ErrCommentPermissionDenied
		}
		result := tx.Where("owner_user_id = ? AND ticket_id = ? AND id = ?", user, id, commentID).Delete(&Comment{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrCommentNotFound
		}
		return nil
	})
}

func (r *GORMRepository) Delete(ctx context.Context, user string, id uint64) error {
	result := r.db.WithContext(ctx).Where("owner_user_id = ? AND id = ?", user, id).Delete(&Ticket{})
	if result.Error != nil {
		return dberror.Classify(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GORMRepository) mutateComment(ctx context.Context, user string, id uint64, updatedBy Actor, now func() time.Time, apply func(*gorm.DB, time.Time) error) (Ticket, error) {
	var out Ticket
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockTicket(tx, user, id); err != nil {
			return err
		}
		updatedAt := now().UTC()
		if err := apply(tx, updatedAt); err != nil {
			return err
		}
		result := tx.Model(&Ticket{}).Where("owner_user_id = ? AND id = ?", user, id).Updates(map[string]any{
			"updated_by_id": updatedBy.ID, "updated_by_name": updatedBy.Name,
			"updated_by_email": updatedBy.Email, "updated_at": updatedAt.UTC(),
		})
		if result.Error != nil {
			return result.Error
		}
		found, err := find(tx, user, id)
		out = found
		return err
	})
	if err != nil {
		return Ticket{}, classify(err)
	}
	return out, nil
}

func lockTicket(tx *gorm.DB, user string, id uint64) (Ticket, error) {
	var out Ticket
	if err := tx.Where("owner_user_id = ? AND id = ?", user, id).Clauses(clause.Locking{Strength: "UPDATE"}).First(&out).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Ticket{}, ErrNotFound
		}
		return Ticket{}, err
	}
	return out, nil
}

func findComment(tx *gorm.DB, user string, ticketID, commentID uint64) (Comment, error) {
	var out Comment
	if err := tx.Where("owner_user_id = ? AND ticket_id = ? AND id = ?", user, ticketID, commentID).First(&out).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Comment{}, ErrCommentNotFound
		}
		return Comment{}, err
	}
	return out, nil
}

func find(db *gorm.DB, user string, id uint64) (Ticket, error) {
	var out Ticket
	if err := db.Where("owner_user_id = ? AND id = ?", user, id).First(&out).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Ticket{}, ErrNotFound
		}
		return Ticket{}, err
	}
	var records []historyRecord
	if err := db.Where("owner_user_id = ? AND ticket_id = ?", user, id).Order("changed_at DESC").Order("id ASC").Find(&records).Error; err != nil {
		return Ticket{}, err
	}
	out.History = groupHistoryRecords(records)
	out.Comments = []Comment{}
	if err := db.Where("owner_user_id = ? AND ticket_id = ?", user, id).Order("created_at DESC").Order("id DESC").Find(&out.Comments).Error; err != nil {
		return Ticket{}, err
	}
	return out, nil
}

// classify はドメインエラーをそのまま通し、それ以外を DB エラーとして分類する。
func classify(err error) error {
	var httpErr *httperror.Error
	if errors.As(err, &httpErr) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrCommentNotFound) || errors.Is(err, ErrCommentPermissionDenied) {
		return err
	}
	return dberror.Classify(err)
}
