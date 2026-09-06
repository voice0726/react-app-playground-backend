package task

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/voice0726/react-app-playground-backend/internal/shared/dberror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/pagination"
	"gorm.io/gorm"
)

type GORMRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GORMRepository { return &GORMRepository{db} }
func (r *GORMRepository) List(c context.Context, organizationID uuid.UUID, options ListOptions) ([]Task, int64, error) {
	var v []Task
	var total int64
	e := r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		query := taskQuery(tx, organizationID, options)
		if err := query.Model(&Task{}).Count(&total).Error; err != nil {
			return dberror.Classify(err)
		}
		query = taskQuery(tx, organizationID, options).Order(pagination.SortExpression(options.Query, taskSorts)).Order(taskIDOrder(options.Query))
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
func (r *GORMRepository) Create(c context.Context, organizationID uuid.UUID, userID string, v Task) (Task, error) {
	v.ID = uuid.New()
	v.OrganizationID = organizationID
	v.OwnerUserID = userID
	e := r.db.WithContext(c).Create(&v).Error
	if e != nil {
		return v, dberror.Classify(e)
	}
	return v, nil
}
func (r *GORMRepository) Update(c context.Context, organizationID, id uuid.UUID, p UpdatePatch) (Task, error) {
	m := map[string]interface{}{}
	if p.Title != nil {
		m["title"] = *p.Title
	}
	if p.Description != nil {
		m["description"] = *p.Description
	}
	if p.Status != nil {
		m["status"] = *p.Status
	}
	m["updated_at"] = gorm.Expr("CURRENT_TIMESTAMP")
	q := r.db.WithContext(c).Model(&Task{}).Where("organization_id = ? AND id = ?", organizationID, id).Updates(m)
	if q.Error != nil {
		return Task{}, dberror.Classify(q.Error)
	}
	if q.RowsAffected == 0 {
		return Task{}, httperror.New(httperror.NotFound, "task_not_found", "Task not found.", nil)
	}
	var v Task
	if e := r.db.WithContext(c).Where("organization_id = ? AND id = ?", organizationID, id).First(&v).Error; e != nil {
		return Task{}, dberror.Classify(e)
	}
	return v, nil
}
func (r *GORMRepository) Delete(c context.Context, organizationID, id uuid.UUID) error {
	q := r.db.WithContext(c).Where("organization_id = ? AND id = ?", organizationID, id).Delete(&Task{})
	if q.Error != nil {
		return dberror.Classify(q.Error)
	}
	if q.RowsAffected == 0 {
		return httperror.New(httperror.NotFound, "task_not_found", "Task not found.", nil)
	}
	return nil
}

var taskSorts = map[string]string{
	"createdAt": "created_at",
	"id":        "id",
	"status":    "status",
	"title":     "title",
	"updatedAt": "updated_at",
}

func taskQuery(tx *gorm.DB, organizationID uuid.UUID, options ListOptions) *gorm.DB {
	query := tx.Where("organization_id = ?", organizationID)
	if options.Query.Q != "" {
		pattern := pagination.LikePattern(options.Query.Q)
		query = query.Where("(title ILIKE ? ESCAPE '\\' OR description ILIKE ? ESCAPE '\\')", pattern, pattern)
	}
	if options.Status != "" {
		query = query.Where("status = ?", options.Status)
	}
	return query
}

func taskIDOrder(query pagination.Query) string {
	if query.Descending {
		return "id DESC"
	}
	return "id ASC"
}
