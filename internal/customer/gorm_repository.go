package customer

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/voice0726/react-app-playground-backend/internal/shared/dberror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/pagination"
	"gorm.io/gorm"
)

type GORMRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GORMRepository { return &GORMRepository{db} }
func (r *GORMRepository) List(ctx context.Context, organizationID uuid.UUID, options ListOptions) ([]Customer, int64, error) {
	var out []Customer
	var total int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := customerQuery(tx, organizationID, options)
		if err := query.Model(&Customer{}).Count(&total).Error; err != nil {
			return dberror.Classify(err)
		}
		query = customerQuery(tx, organizationID, options)
		query = query.Order(pagination.SortExpression(options.Query, customerSorts)).Order(customerIDOrder(options.Query))
		if options.Query.Paginated {
			offset, err := options.Query.Offset()
			if err != nil {
				return httperror.New(httperror.Validation, "pagination_validation", "Invalid pagination.", err)
			}
			query = query.Offset(offset).Limit(options.Query.PageSize)
		}
		if err := query.Find(&out).Error; err != nil {
			return dberror.Classify(err)
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}
func (r *GORMRepository) Find(ctx context.Context, organizationID, id uuid.UUID) (Customer, error) {
	var out Customer
	err := r.db.WithContext(ctx).Where("organization_id = ? AND id = ?", organizationID, id).First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Customer{}, httperror.New(httperror.NotFound, "customer_not_found", "Customer not found.", nil)
	}
	if err != nil {
		return out, dberror.Classify(err)
	}
	return out, nil
}
func (r *GORMRepository) Create(ctx context.Context, organizationID uuid.UUID, userID string, v Customer) (Customer, error) {
	v.ID = uuid.New()
	v.OrganizationID = organizationID
	v.OwnerUserID = userID
	if err := r.db.WithContext(ctx).Create(&v).Error; err != nil {
		return Customer{}, dberror.Classify(err)
	}
	return v, nil
}
func (r *GORMRepository) Update(ctx context.Context, organizationID, id uuid.UUID, p UpdatePatch) (Customer, error) {
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
	q := r.db.WithContext(ctx).Model(&Customer{}).Where("organization_id = ? AND id = ?", organizationID, id).Updates(values)
	if q.Error != nil {
		return Customer{}, dberror.Classify(q.Error)
	}
	if q.RowsAffected == 0 {
		return Customer{}, httperror.New(httperror.NotFound, "customer_not_found", "Customer not found.", nil)
	}
	return r.Find(ctx, organizationID, id)
}
func (r *GORMRepository) Delete(ctx context.Context, organizationID, id uuid.UUID) error {
	q := r.db.WithContext(ctx).Where("organization_id = ? AND id = ?", organizationID, id).Delete(&Customer{})
	if q.Error != nil {
		return dberror.Classify(q.Error)
	}
	if q.RowsAffected == 0 {
		return httperror.New(httperror.NotFound, "customer_not_found", "Customer not found.", nil)
	}
	return nil
}

var customerSorts = map[string]string{
	"createdAt": "created_at",
	"email":     "email",
	"id":        "id",
	"name":      "name",
	"phone":     "phone",
	"status":    "status",
	"updatedAt": "updated_at",
}

func customerQuery(tx *gorm.DB, organizationID uuid.UUID, options ListOptions) *gorm.DB {
	query := tx.Where("organization_id = ?", organizationID)
	if options.Query.Q != "" {
		pattern := pagination.LikePattern(options.Query.Q)
		query = query.Where("(name ILIKE ? ESCAPE '\\' OR email ILIKE ? ESCAPE '\\' OR phone ILIKE ? ESCAPE '\\')", pattern, pattern, pattern)
	}
	if options.Status != "" {
		query = query.Where("status = ?", options.Status)
	}
	return query
}

func customerIDOrder(query pagination.Query) string {
	if query.Descending {
		return "id DESC"
	}
	return "id ASC"
}
