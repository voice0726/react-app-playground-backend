package tickets

import (
	"context"
	"errors"

	"github.com/voice0726/react-app-playground-backend/internal/shared/dberror"
	"gorm.io/gorm"
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

func (r *GORMRepository) Replace(ctx context.Context, user string, ticket Ticket, history []HistoryItem) (Ticket, error) {
	return r.mutate(ctx, user, ticket, func(tx *gorm.DB) error {
		if records := toHistoryRecords(user, ticket.ID, history); len(records) > 0 {
			return tx.Create(&records).Error
		}
		return nil
	})
}

func (r *GORMRepository) AddComment(ctx context.Context, user string, ticket Ticket, comment Comment) (Ticket, error) {
	return r.mutate(ctx, user, ticket, func(tx *gorm.DB) error {
		comment.OwnerUserID = user
		comment.TicketID = ticket.ID
		comment.CreatedAt = comment.CreatedAt.UTC()
		return tx.Create(&comment).Error
	})
}

func (r *GORMRepository) UpdateComment(ctx context.Context, user string, ticket Ticket, comment Comment) (Ticket, error) {
	return r.mutate(ctx, user, ticket, func(tx *gorm.DB) error {
		result := tx.Model(&Comment{}).
			Where("owner_user_id = ? AND ticket_id = ? AND id = ?", user, ticket.ID, comment.ID).
			Update("body", comment.Body)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrCommentNotFound
		}
		return nil
	})
}

func (r *GORMRepository) DeleteComment(ctx context.Context, user string, ticket Ticket, commentID uint64) (Ticket, error) {
	return r.mutate(ctx, user, ticket, func(tx *gorm.DB) error {
		result := tx.Where("owner_user_id = ? AND ticket_id = ? AND id = ?", user, ticket.ID, commentID).Delete(&Comment{})
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

// mutate は 1 トランザクションで ticket 行を更新し、続けて apply を実行して、更新後の ticket を返す。
func (r *GORMRepository) mutate(ctx context.Context, user string, ticket Ticket, apply func(*gorm.DB) error) (Ticket, error) {
	var out Ticket
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&Ticket{}).Where("owner_user_id = ? AND id = ?", user, ticket.ID).Updates(map[string]any{
			"title":            ticket.Title,
			"status":           ticket.Status,
			"assignee":         ticket.Assignee,
			"updated_by_id":    ticket.UpdatedBy.ID,
			"updated_by_name":  ticket.UpdatedBy.Name,
			"updated_by_email": ticket.UpdatedBy.Email,
			"updated_at":       ticket.UpdatedAt.UTC(),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		if err := apply(tx); err != nil {
			return err
		}
		found, err := find(tx, user, ticket.ID)
		out = found
		return err
	})
	if err != nil {
		return Ticket{}, classify(err)
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
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrCommentNotFound) {
		return err
	}
	return dberror.Classify(err)
}
