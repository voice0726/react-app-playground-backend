package deal

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
func (r *GORMRepository) List(c context.Context, organizationID uuid.UUID, options ListOptions) ([]Deal, int64, error) {
	var v []Deal
	var total int64
	e := r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		query := dealQuery(tx, organizationID, options)
		if err := query.Model(&Deal{}).Count(&total).Error; err != nil {
			return dberror.Classify(err)
		}
		query = dealQuery(tx, organizationID, options).
			Order(pagination.SortExpression(options.Query, dealSorts)).Order(dealIDOrder(options.Query))
		if options.Query.Paginated {
			offset, err := options.Query.Offset()
			if err != nil {
				return httperror.New(httperror.Validation, "pagination_validation", "Invalid pagination.", err)
			}
			query = query.Offset(offset).Limit(options.Query.PageSize)
		}
		if err := query.Find(&v).Error; err != nil {
			return dberror.Classify(err)
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if e != nil {
		return nil, 0, e
	}
	return v, total, nil
}
func (r *GORMRepository) Find(c context.Context, organizationID, id uuid.UUID) (Deal, error) {
	var v Deal
	e := r.db.WithContext(c).Where("organization_id = ? AND id = ?", organizationID, id).First(&v).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return v, httperror.New(httperror.NotFound, "deal_not_found", "Deal not found.", nil)
	}
	if e != nil {
		return v, dberror.Classify(e)
	}
	return v, nil
}
func (r *GORMRepository) Create(c context.Context, organizationID uuid.UUID, userID string, v Deal) (Deal, error) {
	v.ID = uuid.New()
	v.OrganizationID = organizationID
	v.OwnerUserID = userID
	e := r.db.WithContext(c).Create(&v).Error
	if e != nil {
		return v, dberror.Classify(e)
	}
	return v, nil
}
func (r *GORMRepository) Update(c context.Context, organizationID, id uuid.UUID, p UpdatePatch) (Deal, error) {
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
	q := r.db.WithContext(c).Model(&Deal{}).Where("organization_id = ? AND id = ?", organizationID, id).Updates(m)
	if q.Error != nil {
		return Deal{}, dberror.Classify(q.Error)
	}
	if q.RowsAffected == 0 {
		return Deal{}, httperror.New(httperror.NotFound, "deal_not_found", "Deal not found.", nil)
	}
	return r.Find(c, organizationID, id)
}

var dealSorts = map[string]string{
	"customerId": "customer_id",
	"amount":     "amount",
	"createdAt":  "created_at",
	"id":         "id",
	"status":     "status",
	"title":      "title",
	"updatedAt":  "updated_at",
}

func dealQuery(tx *gorm.DB, organizationID uuid.UUID, options ListOptions) *gorm.DB {
	query := tx.Where("organization_id = ?", organizationID)
	if options.Query.Q != "" {
		query = query.Where("title ILIKE ? ESCAPE '\\'", pagination.LikePattern(options.Query.Q))
	}
	if options.Status != "" {
		query = query.Where("status = ?", options.Status)
	}
	if options.CustomerID != nil {
		query = query.Where("customer_id = ?", *options.CustomerID)
	}
	return query
}

func dealIDOrder(query pagination.Query) string {
	if query.Descending {
		return "id DESC"
	}
	return "id ASC"
}
