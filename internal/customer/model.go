package customer

import (
	"github.com/google/uuid"
	"time"
)

type Customer struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrganizationID uuid.UUID `gorm:"type:uuid;not null;index"`
	OwnerUserID    string    `gorm:"column:owner_user_id"`
	Name           string
	Email          string
	Phone          string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (Customer) TableName() string { return "customers" }

type Response struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

func toResponse(v Customer) Response {
	return Response{v.ID.String(), v.Name, v.Email, v.Phone, v.Status, v.CreatedAt}
}
