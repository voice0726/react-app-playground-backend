package deal

import (
	"github.com/google/uuid"
	"time"
)

type Deal struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	OwnerUserID string
	CustomerID  uuid.UUID
	Title       string
	Amount      int64
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Deal) TableName() string { return "deals" }

type Response struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	CustomerID string    `json:"customerId"`
	Amount     int64     `json:"amount"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"createdAt"`
}

func response(v Deal) Response {
	return Response{v.ID.String(), v.Title, v.CustomerID.String(), v.Amount, v.Status, v.CreatedAt}
}
