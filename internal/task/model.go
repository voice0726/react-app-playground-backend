package task

import (
	"github.com/google/uuid"
	"time"
)

type Task struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	OwnerUserID string
	Title       string
	Description string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Task) TableName() string { return "tasks" }

type Response struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}

func response(v Task) Response {
	return Response{v.ID.String(), v.Title, v.Description, v.Status, v.CreatedAt}
}
