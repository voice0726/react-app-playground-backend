package organization

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/voice0726/react-app-playground-backend/internal/shared/dberror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GORMRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GORMRepository { return &GORMRepository{db: db} }

type organizationWithRole struct {
	Organization
	Role string `gorm:"column:member_role"`
}

func (r *GORMRepository) ListForUser(ctx context.Context, userID string) ([]Organization, error) {
	var rows []organizationWithRole
	err := r.db.WithContext(ctx).
		Table("organizations AS organizations").
		Select("organizations.id, organizations.name, organizations.created_at, organizations.updated_at, organization_members.role AS member_role").
		Joins("JOIN organization_members ON organization_members.organization_id = organizations.id").
		Where("organization_members.user_id = ?", userID).
		Order("organizations.name ASC").Order("organizations.id ASC").Find(&rows).Error
	if err != nil {
		return nil, dberror.Classify(err)
	}
	out := make([]Organization, 0, len(rows))
	for _, row := range rows {
		row.Organization.Role = row.Role
		out = append(out, row.Organization)
	}
	return out, nil
}

func (r *GORMRepository) Create(ctx context.Context, userID, name string) (Organization, error) {
	var organization Organization
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		organization = Organization{ID: uuid.New(), Name: name}
		if err := tx.Create(&organization).Error; err != nil {
			return dberror.Classify(err)
		}
		member := Member{OrganizationID: organization.ID, UserID: userID, Role: RoleAdmin}
		if err := tx.Create(&member).Error; err != nil {
			return dberror.Classify(err)
		}
		organization.Role = RoleAdmin
		return nil
	})
	if err != nil {
		return Organization{}, err
	}
	return organization, nil
}

func (r *GORMRepository) Role(ctx context.Context, userID string, organizationID uuid.UUID) (string, error) {
	var member Member
	err := r.db.WithContext(ctx).Where("organization_id = ? AND user_id = ?", organizationID, userID).First(&member).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", httperror.New(httperror.NotFound, "organization_not_found", "Organization not found.", nil)
	}
	if err != nil {
		return "", dberror.Classify(err)
	}
	return member.Role, nil
}

func (r *GORMRepository) ListMembers(ctx context.Context, userID string, organizationID uuid.UUID) ([]Member, error) {
	if _, err := r.Role(ctx, userID, organizationID); err != nil {
		return nil, err
	}
	var members []Member
	if err := r.db.WithContext(ctx).Where("organization_id = ?", organizationID).Order("user_id ASC").Find(&members).Error; err != nil {
		return nil, dberror.Classify(err)
	}
	return members, nil
}

func (r *GORMRepository) AddMember(ctx context.Context, actorID string, organizationID uuid.UUID, userID, role string) (Member, error) {
	var member Member
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.lockOrganization(tx, organizationID); err != nil {
			return err
		}
		if err := r.requireAdmin(tx, actorID, organizationID); err != nil {
			return err
		}
		member = Member{OrganizationID: organizationID, UserID: userID, Role: role}
		if err := tx.Create(&member).Error; err != nil {
			return dberror.Classify(err)
		}
		return nil
	})
	if err != nil {
		return Member{}, err
	}
	return member, nil
}

func (r *GORMRepository) UpdateMemberRole(ctx context.Context, actorID string, organizationID uuid.UUID, userID, role string) (Member, error) {
	var member Member
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.lockOrganization(tx, organizationID); err != nil {
			return err
		}
		if err := r.requireAdmin(tx, actorID, organizationID); err != nil {
			return err
		}
		if err := tx.Where("organization_id = ? AND user_id = ?", organizationID, userID).First(&member).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return httperror.New(httperror.NotFound, "member_not_found", "Organization member not found.", nil)
			}
			return dberror.Classify(err)
		}
		if member.Role == RoleAdmin && role != RoleAdmin {
			var admins int64
			if err := tx.Model(&Member{}).Where("organization_id = ? AND role = ?", organizationID, RoleAdmin).Count(&admins).Error; err != nil {
				return dberror.Classify(err)
			}
			if admins <= 1 {
				return httperror.New(httperror.Conflict, "last_admin_required", "An organization must have at least one administrator.", nil)
			}
		}
		if err := tx.Model(&Member{}).Where("organization_id = ? AND user_id = ?", organizationID, userID).Updates(map[string]interface{}{"role": role, "updated_at": gorm.Expr("CURRENT_TIMESTAMP")}).Error; err != nil {
			return dberror.Classify(err)
		}
		member.Role = role
		return nil
	})
	if err != nil {
		return Member{}, err
	}
	return member, nil
}

func (r *GORMRepository) RemoveMember(ctx context.Context, actorID string, organizationID uuid.UUID, userID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.lockOrganization(tx, organizationID); err != nil {
			return err
		}
		if err := r.requireAdmin(tx, actorID, organizationID); err != nil {
			return err
		}
		var member Member
		if err := tx.Where("organization_id = ? AND user_id = ?", organizationID, userID).First(&member).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return httperror.New(httperror.NotFound, "member_not_found", "Organization member not found.", nil)
			}
			return dberror.Classify(err)
		}
		if member.Role == RoleAdmin {
			var admins int64
			if err := tx.Model(&Member{}).Where("organization_id = ? AND role = ?", organizationID, RoleAdmin).Count(&admins).Error; err != nil {
				return dberror.Classify(err)
			}
			if admins <= 1 {
				return httperror.New(httperror.Conflict, "last_admin_required", "An organization must have at least one administrator.", nil)
			}
		}
		result := tx.Where("organization_id = ? AND user_id = ?", organizationID, userID).Delete(&Member{})
		if result.Error != nil {
			return dberror.Classify(result.Error)
		}
		return nil
	})
}

func (r *GORMRepository) lockOrganization(tx *gorm.DB, organizationID uuid.UUID) error {
	var organization Organization
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", organizationID).First(&organization).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return httperror.New(httperror.NotFound, "organization_not_found", "Organization not found.", nil)
	}
	if err != nil {
		return dberror.Classify(err)
	}
	return nil
}

func (r *GORMRepository) requireAdmin(tx *gorm.DB, actorID string, organizationID uuid.UUID) error {
	var member Member
	err := tx.Where("organization_id = ? AND user_id = ?", organizationID, actorID).First(&member).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return httperror.New(httperror.NotFound, "organization_not_found", "Organization not found.", nil)
	}
	if err != nil {
		return dberror.Classify(err)
	}
	if member.Role != RoleAdmin {
		return httperror.New(httperror.Forbidden, "organization_admin_required", "Administrator permission is required.", nil)
	}
	return nil
}
