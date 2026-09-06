package invoice

import (
	"github.com/google/uuid"
	"time"
)

type Invoice struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrganizationID uuid.UUID `gorm:"type:uuid;not null;index"`
	OwnerUserID    string
	CustomerID     uuid.UUID
	Number         string
	Amount         int64
	IssueDate      time.Time
	DueDate        time.Time
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Summary struct {
	DraftCount        int64 `json:"draftCount"`
	OutstandingAmount int64 `json:"outstandingAmount"`
	OverdueCount      int64 `json:"overdueCount"`
	OverdueAmount     int64 `json:"overdueAmount"`
}

func (Invoice) TableName() string { return "invoices" }

type Response struct {
	ID         string    `json:"id"`
	Number     string    `json:"number"`
	CustomerID string    `json:"customerId"`
	Amount     int64     `json:"amount"`
	IssueDate  string    `json:"issueDate"`
	DueDate    string    `json:"dueDate"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"createdAt"`
}

func response(v Invoice) Response {
	return Response{v.ID.String(), v.Number, v.CustomerID.String(), v.Amount, v.IssueDate.Format("2006-01-02"), v.DueDate.Format("2006-01-02"), v.Status, v.CreatedAt}
}
