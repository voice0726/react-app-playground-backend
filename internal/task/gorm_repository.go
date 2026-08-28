package task

import (
	"context"
	"github.com/google/uuid"
	"github.com/voice0726/react-app-playground-backend/internal/shared/dberror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
	"gorm.io/gorm"
)

type GORMRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GORMRepository { return &GORMRepository{db} }
func (r *GORMRepository) List(c context.Context, u string) ([]Task, error) {
	var v []Task
	e := r.db.WithContext(c).Where("owner_user_id = ?", u).Order("created_at ASC").Order("id ASC").Find(&v).Error
	if e != nil {
		return v, dberror.Classify(e)
	}
	return v, nil
}
func (r *GORMRepository) Create(c context.Context, u string, v Task) (Task, error) {
	v.ID = uuid.New()
	v.OwnerUserID = u
	e := r.db.WithContext(c).Create(&v).Error
	if e != nil {
		return v, dberror.Classify(e)
	}
	return v, nil
}
func (r *GORMRepository) Update(c context.Context, u string, id uuid.UUID, p UpdatePatch) (Task, error) {
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
	q := r.db.WithContext(c).Model(&Task{}).Where("owner_user_id = ? AND id = ?", u, id).Updates(m)
	if q.Error != nil {
		return Task{}, dberror.Classify(q.Error)
	}
	if q.RowsAffected == 0 {
		return Task{}, httperror.New(httperror.NotFound, "task_not_found", "Task not found.", nil)
	}
	var v Task
	if e := r.db.WithContext(c).Where("owner_user_id = ? AND id = ?", u, id).First(&v).Error; e != nil {
		return Task{}, dberror.Classify(e)
	}
	return v, nil
}
func (r *GORMRepository) Delete(c context.Context, u string, id uuid.UUID) error {
	q := r.db.WithContext(c).Where("owner_user_id = ? AND id = ?", u, id).Delete(&Task{})
	if q.Error != nil {
		return dberror.Classify(q.Error)
	}
	if q.RowsAffected == 0 {
		return httperror.New(httperror.NotFound, "task_not_found", "Task not found.", nil)
	}
	return nil
}
