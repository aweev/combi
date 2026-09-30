package organization

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/organization"
	"github.com/combi/pkg/log"
)

var _ organization.RoleAdminService = (*RoleService)(nil)

type RoleService struct {
	repo          organization.Repository
	auditRecorder audit.EventRecorder
	logger        log.Logger
	// Reusing the main service for permission checks to avoid code duplication
	orgSvc *Service
}

func NewRoleService(repo organization.Repository, audit audit.EventRecorder, logger log.Logger, orgSvc *Service) *RoleService {
	return &RoleService{
		repo:          repo,
		auditRecorder: audit,
		logger:        logger,
		orgSvc:        orgSvc,
	}
}

func (s *RoleService) ListRoles(ctx context.Context, orgID uuid.UUID) ([]*organization.Role, error) {
	if err := s.orgSvc.requirePermission(ctx, orgID, organization.PermRolesList); err != nil {
		return nil, err
	}
	return s.repo.ListRole(ctx, orgID)
}

func (s *RoleService) CreateRole(ctx context.Context, orgID uuid.UUID, params organization.CreateRoleParams) (*organization.Role, error) {
	if err := s.orgSvc.requirePermission(ctx, orgID, organization.PermRolesCreate); err != nil {
		return nil, err
	}

	role := &organization.Role{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           params.Name,
		Description:    params.Description,
		IsSystemRole:   false,
		Permissions:    params.Permissions,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	if err := s.repo.CreateRole(ctx, role); err != nil {
		return nil, s.orgSvc.wrapError(err)
	}

	s.audit(ctx, core.EventOrganizationUpdated, orgID, role.ID, "role", map[string]interface{}{"action": "create_role", "role_name": role.Name})
	return role, nil
}

func (s *RoleService) UpdateRole(ctx context.Context, orgID, roleID uuid.UUID, params organization.UpdateRoleParams) (*organization.Role, error) {
	if err := s.orgSvc.requirePermission(ctx, orgID, organization.PermRolesUpdate); err != nil {
		return nil, err
	}

	var updated *organization.Role

	err := s.repo.WithTx(ctx, func(txRepo organization.Repository) error {
		role, err := txRepo.GetRoleByID(ctx, roleID)
		if err != nil {
			return err
		}

		if role.OrganizationID != orgID {
			return organization.ErrRoleNotFound
		}

		if role.IsSystemRole {
			return organization.ErrSystemRoleImmutable
		}

		if params.Name != nil {
			role.Name = *params.Name
		}
		if params.Description != nil {
			role.Description = *params.Description
		}
		if params.Permissions != nil {
			role.Permissions = params.Permissions
		}

		if err := txRepo.UpdateRole(ctx, role); err != nil {
			return err
		}
		updated = role
		return nil
	})

	if err != nil {
		return nil, s.orgSvc.wrapError(err)
	}

	s.audit(ctx, core.EventOrganizationUpdated, orgID, roleID, "role", map[string]interface{}{"action": "update_role"})
	return updated, nil
}

func (s *RoleService) DeleteRole(ctx context.Context, orgID, roleID uuid.UUID) error {
	if err := s.orgSvc.requirePermission(ctx, orgID, organization.PermRolesDelete); err != nil {
		return err
	}

	role, err := s.repo.GetRoleByID(ctx, roleID)
	if err != nil {
		return s.orgSvc.wrapError(err)
	}
	if role.OrganizationID != orgID {
		return organization.ErrRoleNotFound
	}
	if role.IsSystemRole {
		return organization.ErrSystemRoleImmutable
	}

	if err := s.repo.DeleteRole(ctx, roleID); err != nil {
		return s.orgSvc.wrapError(err)
	}

	s.audit(ctx, core.EventOrganizationUpdated, orgID, roleID, "role", map[string]interface{}{"action": "delete_role"})
	return nil
}

func (s *RoleService) audit(ctx context.Context, action core.EventAction, orgID, targetID uuid.UUID, targetType string, meta map[string]interface{}) {
	// Re-use logic for consistency
	s.orgSvc.audit(ctx, action, orgID, targetID, targetType, meta)
}
