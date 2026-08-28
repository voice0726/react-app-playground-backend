package invoice

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/voice0726/react-app-playground-backend/internal/shared/dberror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GORMRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GORMRepository { return &GORMRepository{db} }
func (r *GORMRepository) List(c context.Context, u string) ([]Invoice, error) {
	var v []Invoice
	e := r.db.WithContext(c).Where("owner_user_id = ?", u).Order("created_at ASC").Order("id ASC").Find(&v).Error
	if e != nil {
		return v, dberror.Classify(e)
	}
	return v, nil
}
func (r *GORMRepository) Find(c context.Context, u string, id uuid.UUID) (Invoice, error) {
	var v Invoice
	e := r.db.WithContext(c).Where("owner_user_id = ? AND id = ?", u, id).First(&v).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return v, httperror.New(httperror.NotFound, "invoice_not_found", "Invoice not found.", nil)
	}
	if e != nil {
		return v, dberror.Classify(e)
	}
	return v, nil
}
func (r *GORMRepository) Create(c context.Context, u string, v Invoice) (Invoice, error) {
	v.ID = uuid.New()
	v.OwnerUserID = u
	e := r.db.WithContext(c).Create(&v).Error
	if e != nil {
		return v, dberror.Classify(e)
	}
	return v, nil
}
func (r *GORMRepository) UpdateLocked(c context.Context, u string, id uuid.UUID, apply func(Invoice) (Invoice, error)) (Invoice, error) {
	var out Invoice
	e := r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		if e := tx.Where("owner_user_id = ? AND id = ?", u, id).Clauses(clause.Locking{Strength: "UPDATE"}).First(&out).Error; e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return httperror.New(httperror.NotFound, "invoice_not_found", "Invoice not found.", nil)
			}
			return dberror.Classify(e)
		}
		next, e := apply(out)
		if e != nil {
			return e
		}
		next.OwnerUserID = u
		next.ID = id
		next.UpdatedAt = out.UpdatedAt
		if e = tx.Save(&next).Error; e != nil {
			return dberror.Classify(e)
		}
		out = next
		return nil
	})
	if e != nil {
		var httpErr *httperror.Error
		if errors.As(e, &httpErr) || errors.Is(e, ErrTransition) {
			return out, e
		}
		return out, dberror.Classify(e)
	}
	return out, nil
}
