package customer

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
func (r *GORMRepository) List(ctx context.Context, user string) ([]Customer, error) {
	var out []Customer
	err := r.db.WithContext(ctx).Where("owner_user_id = ?", user).Order("created_at ASC").Order("id ASC").Find(&out).Error
	if err != nil {
		return out, dberror.Classify(err)
	}
	return out, nil
}
func (r *GORMRepository) Find(ctx context.Context, user string, id uuid.UUID) (Customer, error) {
	var out Customer
	err := r.db.WithContext(ctx).Where("owner_user_id = ? AND id = ?", user, id).First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Customer{}, httperror.New(httperror.NotFound, "customer_not_found", "Customer not found.", nil)
	}
	if err != nil {
		return out, dberror.Classify(err)
	}
	return out, nil
}
func (r *GORMRepository) Create(ctx context.Context, user string, v Customer) (Customer, error) {
	v.ID = uuid.New()
	v.OwnerUserID = user
	if err := r.db.WithContext(ctx).Create(&v).Error; err != nil {
		return Customer{}, dberror.Classify(err)
	}
	return v, nil
}
func (r *GORMRepository) Update(ctx context.Context, user string, id uuid.UUID, p UpdatePatch) (Customer, error) {
	values := map[string]interface{}{}
	if p.Name != nil {
		values["name"] = *p.Name
	}
	if p.Email != nil {
		values["email"] = *p.Email
	}
	if p.Phone != nil {
		values["phone"] = *p.Phone
	}
	if p.Status != nil {
		values["status"] = *p.Status
	}
	values["updated_at"] = gorm.Expr("CURRENT_TIMESTAMP")
	q := r.db.WithContext(ctx).Model(&Customer{}).Where("owner_user_id = ? AND id = ?", user, id).Updates(values)
	if q.Error != nil {
		return Customer{}, dberror.Classify(q.Error)
	}
	if q.RowsAffected == 0 {
		return Customer{}, httperror.New(httperror.NotFound, "customer_not_found", "Customer not found.", nil)
	}
	return r.Find(ctx, user, id)
}
func (r *GORMRepository) Delete(ctx context.Context, user string, id uuid.UUID) error {
	q := r.db.WithContext(ctx).Where("owner_user_id = ? AND id = ?", user, id).Delete(&Customer{})
	if q.Error != nil {
		return dberror.Classify(q.Error)
	}
	if q.RowsAffected == 0 {
		return httperror.New(httperror.NotFound, "customer_not_found", "Customer not found.", nil)
	}
	return nil
}
