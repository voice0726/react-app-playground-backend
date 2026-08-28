package deal

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/voice0726/react-app-playground-backend/internal/shared/dberror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
	"gorm.io/gorm"
)

type GORMRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GORMRepository { return &GORMRepository{db} }
func (r *GORMRepository) List(c context.Context, u string) ([]Deal, error) {
	var v []Deal
	e := r.db.WithContext(c).Where("owner_user_id = ?", u).Order("created_at ASC").Order("id ASC").Find(&v).Error
	if e != nil {
		return v, dberror.Classify(e)
	}
	return v, nil
}
func (r *GORMRepository) Find(c context.Context, u string, id uuid.UUID) (Deal, error) {
	var v Deal
	e := r.db.WithContext(c).Where("owner_user_id = ? AND id = ?", u, id).First(&v).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return v, httperror.New(httperror.NotFound, "deal_not_found", "Deal not found.", nil)
	}
	if e != nil {
		return v, dberror.Classify(e)
	}
	return v, nil
}
func (r *GORMRepository) Create(c context.Context, u string, v Deal) (Deal, error) {
	v.ID = uuid.New()
	v.OwnerUserID = u
	e := r.db.WithContext(c).Create(&v).Error
	if e != nil {
		return v, dberror.Classify(e)
	}
	return v, nil
}
func (r *GORMRepository) Update(c context.Context, u string, id uuid.UUID, p UpdatePatch) (Deal, error) {
	m := map[string]interface{}{}
	if p.Title != nil {
		m["title"] = *p.Title
	}
	if p.CustomerID != nil {
		m["customer_id"] = *p.CustomerID
	}
	if p.Amount != nil {
		m["amount"] = *p.Amount
	}
	if p.Status != nil {
		m["status"] = *p.Status
	}
	m["updated_at"] = gorm.Expr("CURRENT_TIMESTAMP")
	q := r.db.WithContext(c).Model(&Deal{}).Where("owner_user_id = ? AND id = ?", u, id).Updates(m)
	if q.Error != nil {
		return Deal{}, dberror.Classify(q.Error)
	}
	if q.RowsAffected == 0 {
		return Deal{}, httperror.New(httperror.NotFound, "deal_not_found", "Deal not found.", nil)
	}
	return r.Find(c, u, id)
}
