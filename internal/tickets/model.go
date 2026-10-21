package tickets

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusAll    Status = "all"
	StatusOpen   Status = "open"
	StatusClosed Status = "closed"
)

type SortBy string

const (
	SortByID        SortBy = "id"
	SortByCreatedAt SortBy = "created_at"
	SortByUpdatedAt SortBy = "updated_at"
)

type SortOrder string

const (
	SortOrderAsc SortOrder = "asc"
	SortOrderDsc SortOrder = "dsc"
)

// Actor は操作者のスナップショット。auth-gateway のユーザー情報を操作時点の値で保存する。
type Actor struct {
	ID    string `gorm:"column:id"`
	Name  string `gorm:"column:name"`
	Email string `gorm:"column:email"`
}

type Ticket struct {
	ID             uint64 `gorm:"primaryKey;autoIncrement"`
	OrganizationID uuid.UUID
	OwnerUserID    string
	Title          string
	Status         Status
	Assignee       *string
	CreatedBy      Actor `gorm:"embedded;embeddedPrefix:created_by_"`
	UpdatedBy      Actor `gorm:"embedded;embeddedPrefix:updated_by_"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	History        []HistoryItem `gorm:"-"`
	Comments       []Comment     `gorm:"-"`
}

func (Ticket) TableName() string { return "tickets" }

type Comment struct {
	ID             uint64 `gorm:"primaryKey;autoIncrement"`
	OrganizationID uuid.UUID
	TicketID       uint64
	Body           string
	CreatedBy      Actor `gorm:"embedded;embeddedPrefix:created_by_"`
	CreatedAt      time.Time
}

func (Comment) TableName() string { return "ticket_comments" }

type HistoryField string

const HistoryFieldStatus HistoryField = "status"

type HistoryChange struct {
	Field  HistoryField
	Before *string
	After  *string
}

// HistoryItem は 1 回の更新操作(operation)にまとめた履歴。
type HistoryItem struct {
	OperationID string
	Actor       Actor
	ChangedAt   time.Time
	Changes     []HistoryChange
}

// historyRecord は ticket_histories の 1 行(1 フィールドの差分)。
type historyRecord struct {
	ID             uint64 `gorm:"primaryKey;autoIncrement"`
	OrganizationID uuid.UUID
	TicketID       uint64
	OperationID    string
	Field          string
	BeforeValue    *string
	AfterValue     *string
	ChangedBy      Actor `gorm:"embedded;embeddedPrefix:changed_by_"`
	ChangedAt      time.Time
}

func (historyRecord) TableName() string { return "ticket_histories" }

type ListQuery struct {
	Q         string
	Status    Status
	SortBy    SortBy
	SortOrder SortOrder
	Page      int
	PageSize  int
}

type ListResult struct {
	Items []Ticket
	Total int
}
