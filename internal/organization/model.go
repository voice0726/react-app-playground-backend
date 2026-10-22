package organization

import (
	"time"

	"github.com/google/uuid"
)

const (
	RoleAdmin  = "admin"
	RoleEditor = "editor"
	RoleViewer = "viewer"
)

func ValidRole(role string) bool {
	return role == RoleAdmin || role == RoleEditor || role == RoleViewer
}

type Organization struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
	Role      string `gorm:"-"`
}

func (Organization) TableName() string { return "organizations" }

type Member struct {
	OrganizationID uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID         string    `gorm:"primaryKey;column:user_id"`
	Role           string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (Member) TableName() string { return "organization_members" }

type OrganizationResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

func organizationResponse(value Organization) OrganizationResponse {
	return OrganizationResponse{ID: value.ID.String(), Name: value.Name, Role: value.Role}
}

type MemberResponse struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
}

func memberResponse(value Member) MemberResponse {
	return MemberResponse{UserID: value.UserID, Role: value.Role}
}
