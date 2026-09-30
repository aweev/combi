package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/organization"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/log"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var _ organization.Repository = (*OrganizationRepository)(nil)

type OrganizationRepository struct {
	db     *sqlx.DB
	runner sqlx.ExtContext
	logger log.Logger
	sq     squirrel.StatementBuilderType
}

func NewOrganizationRepository(db *sqlx.DB, logger log.Logger) *OrganizationRepository {
	return &OrganizationRepository{
		db:     db,
		runner: db,
		logger: logger.With(log.String("component", "organization_repository")),
		sq:     squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

func (r *OrganizationRepository) WithTx(ctx context.Context, fn func(txRepo organization.Repository) error) (err error) {
	if _, ok := r.runner.(*sqlx.Tx); ok {
		return errors.New("cannot begin transaction from within another transaction")
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()
	txRepo := &OrganizationRepository{
		db:     r.db,
		runner: tx,
		logger: r.logger,
		sq:     r.sq,
	}
	return fn(txRepo)
}

// --- OrganizationRepository Implementation ---

func (r *OrganizationRepository) CreateOrganization(ctx context.Context, org *organization.Organization) error {
	dbModel := dbOrganizationFromDomain(org)
	query, args, err := r.sq.Insert("organizations").
		Columns("id", "owner_id", "name", "slug", "type", "status", "avatar_id", "version", "created_at", "updated_at").
		Values(dbModel.ID, dbModel.OwnerID, dbModel.Name, dbModel.Slug, dbModel.Type, dbModel.Status, dbModel.AvatarID, dbModel.Version, dbModel.CreatedAt, dbModel.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return organization.ErrOrganizationAlreadyExists
		}
		return err
	}
	return nil
}

func (r *OrganizationRepository) GetOrganizationByID(ctx context.Context, id uuid.UUID) (*organization.Organization, error) {
	return r.getOrganization(ctx, squirrel.Eq{"id": id})
}

func (r *OrganizationRepository) GetOrganizationBySlug(ctx context.Context, slug string) (*organization.Organization, error) {
	return r.getOrganization(ctx, squirrel.Eq{"slug": slug})
}

func (r *OrganizationRepository) GetOrganizationByDomain(ctx context.Context, domain string) (*organization.Organization, error) {
	query, args, _ := r.sq.Select("o.*").
		From("organizations o").
		Join("organization_domains od ON o.id = od.organization_id").
		Where(squirrel.Eq{"od.domain": domain}).
		ToSql()

	var dbOrg dbOrganization
	if err := sqlx.GetContext(ctx, r.runner, &dbOrg, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, organization.ErrOrganizationNotFound
		}
		return nil, err
	}
	return dbOrg.toDomain(), nil
}

func (r *OrganizationRepository) getOrganization(ctx context.Context, pred interface{}) (*organization.Organization, error) {
	query, args, _ := r.sq.Select("*").From("organizations").Where(pred).ToSql()
	var dbOrg dbOrganization
	if err := sqlx.GetContext(ctx, r.runner, &dbOrg, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, organization.ErrOrganizationNotFound
		}
		return nil, err
	}
	return dbOrg.toDomain(), nil
}

func (r *OrganizationRepository) UpdateOrganization(ctx context.Context, org *organization.Organization) error {
	org.UpdatedAt = time.Now().UTC()
	org.Version++
	dbModel := dbOrganizationFromDomain(org)

	query, args, err := r.sq.Update("organizations").
		Set("name", dbModel.Name).
		Set("slug", dbModel.Slug).
		Set("status", org.Status).
		Set("avatar_id", dbModel.AvatarID).
		Set("owner_id", dbModel.OwnerID).
		Set("version", dbModel.Version).
		Set("updated_at", dbModel.UpdatedAt).
		Where(squirrel.Eq{"id": dbModel.ID}).
		ToSql()
	if err != nil {
		return err
	}
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return organization.ErrOrganizationAlreadyExists
		}
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return organization.ErrOrganizationNotFound
	}
	return nil
}

func (r *OrganizationRepository) DeleteOrganization(ctx context.Context, id uuid.UUID) error {
	query, args, _ := r.sq.Delete("organizations").Where(squirrel.Eq{"id": id}).ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return organization.ErrOrganizationNotFound
	}
	return nil
}

func (r *OrganizationRepository) ListOrganizationsForUser(ctx context.Context, userID uuid.UUID) ([]*organization.Organization, error) {
	// Join org_members to find orgs
	query, args, _ := r.sq.Select("o.*").
		From("organizations o").
		Join("org_members om ON o.id = om.organization_id").
		Where(squirrel.Eq{"om.user_id": userID}).
		OrderBy("o.name ASC").
		ToSql()

	var dbOrgs []dbOrganization
	if err := sqlx.SelectContext(ctx, r.runner, &dbOrgs, query, args...); err != nil {
		return nil, err
	}

	orgs := make([]*organization.Organization, len(dbOrgs))
	for i, o := range dbOrgs {
		orgs[i] = o.toDomain()
	}
	return orgs, nil
}

// --- MemberRepository Implementation ---

func (r *OrganizationRepository) CreateMember(ctx context.Context, member *organization.Member) error {
	query, args, _ := r.sq.Insert("org_members").
		Columns("organization_id", "user_id", "role_id", "joined_at").
		Values(member.OrganizationID, member.UserID, member.RoleID, member.JoinedAt).
		ToSql()
	_, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return organization.ErrUserIsAlreadyMember
		}
		return err
	}
	return nil
}

func (r *OrganizationRepository) GetMember(ctx context.Context, orgID, userID uuid.UUID) (*organization.Member, error) {
	query, args, _ := r.sq.Select("*").From("org_members").
		Where(squirrel.Eq{"organization_id": orgID, "user_id": userID}).ToSql()
	var dbm dbMember
	if err := sqlx.GetContext(ctx, r.runner, &dbm, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, organization.ErrMemberNotFound
		}
		return nil, err
	}
	return dbm.toDomain(), nil
}

func (r *OrganizationRepository) GetMemberWithDetails(ctx context.Context, orgID, userID uuid.UUID) (*organization.MemberDetails, error) {
	// Explicit aliasing to match struct tags
	query, args, _ := r.sq.Select(
		"om.joined_at",
		"u.id as u_id", "u.first_name as u_first_name", "u.last_name as u_last_name", "u.email as u_email",
		"u.status as u_status", "u.created_at as u_created_at", "u.updated_at as u_updated_at",
		"r.id as r_id", "r.organization_id as r_organization_id", "r.name as r_name", "r.description as r_description",
		"r.is_system_role as r_is_system_role", "r.permissions as r_permissions", "r.created_at as r_created_at", "r.updated_at as r_updated_at",
	).
		From("org_members om").
		Join("users u ON om.user_id = u.id").
		Join("org_roles r ON om.role_id = r.id").
		Where(squirrel.Eq{"om.organization_id": orgID, "om.user_id": userID}).
		ToSql()

	var row dbMemberDetailsRow
	if err := sqlx.GetContext(ctx, r.runner, &row, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, organization.ErrMemberNotFound
		}
		return nil, err
	}
	return row.toDomain()
}

func (r *OrganizationRepository) ListMemberWithDetails(ctx context.Context, orgID uuid.UUID) ([]*organization.MemberDetails, error) {
	query, args, _ := r.sq.Select(
		"om.joined_at",
		"u.id as user_id", "u.first_name", "u.last_name", "u.email", "u.status", "u.created_at", "u.updated_at",
		"r.id as role_id", "r.organization_id", "r.name as role_name", "r.description", "r.is_system_role", "r.permissions", "r.created_at as role_created_at", "r.updated_at as role_updated_at",
	).
		From("org_members om").
		Join("users u ON om.user_id = u.id").
		Join("org_roles r ON om.role_id = r.id").
		Where(squirrel.Eq{"om.organization_id": orgID}).
		OrderBy("u.last_name ASC, u.first_name ASC").
		ToSql()

	var rows []dbMemberDetailsRow
	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, err
	}

	details := make([]*organization.MemberDetails, len(rows))
	for i, row := range rows {
		d, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		details[i] = d
	}
	return details, nil
}

func (r *OrganizationRepository) UpdateMember(ctx context.Context, member *organization.Member) error {
	query, args, _ := r.sq.Update("org_members").
		Set("role_id", member.RoleID).
		Where(squirrel.Eq{"organization_id": member.OrganizationID, "user_id": member.UserID}).
		ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return organization.ErrMemberNotFound
	}
	return nil
}

func (r *OrganizationRepository) DeleteMember(ctx context.Context, orgID, userID uuid.UUID) error {
	query, args, _ := r.sq.Delete("org_members").
		Where(squirrel.Eq{"organization_id": orgID, "user_id": userID}).ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return organization.ErrMemberNotFound
	}
	return nil
}

func (r *OrganizationRepository) CountMembers(ctx context.Context, orgID uuid.UUID) (int, error) {
	query, args, _ := r.sq.Select("COUNT(*)").From("org_members").Where(squirrel.Eq{"organization_id": orgID}).ToSql()
	var count int
	err := r.runner.QueryRowxContext(ctx, query, args...).Scan(&count)
	return count, err
}

func (r *OrganizationRepository) CountMembersOwners(ctx context.Context, orgID uuid.UUID) (int, error) {
	// Assuming 'Owner' is a role name or logic relies on is_system_role for owners?
	// Usually "Owner" is the name of the role.
	query, args, _ := r.sq.Select("COUNT(*)").
		From("org_members om").
		Join("org_roles r ON om.role_id = r.id").
		Where(squirrel.Eq{"om.organization_id": orgID, "r.name": "Owner"}).
		ToSql()
	var count int
	err := r.runner.QueryRowxContext(ctx, query, args...).Scan(&count)
	return count, err
}

// --- RoleRepository Implementation ---

func (r *OrganizationRepository) CreateRole(ctx context.Context, role *organization.Role) error {
	dbModel := dbRoleFromDomain(role)
	query, args, err := r.sq.Insert("org_roles").
		Columns("id", "organization_id", "name", "description", "is_system_role", "permissions", "created_at", "updated_at").
		Values(dbModel.ID, dbModel.OrganizationID, dbModel.Name, dbModel.Description, dbModel.IsSystemRole, dbModel.Permissions, dbModel.CreatedAt, dbModel.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return organization.ErrRoleAlreadyExists
		}
		return err
	}
	return nil
}

func (r *OrganizationRepository) GetRoleByID(ctx context.Context, id uuid.UUID) (*organization.Role, error) {
	query, args, _ := r.sq.Select("*").From("org_roles").Where(squirrel.Eq{"id": id}).ToSql()
	var dbr dbRole
	if err := sqlx.GetContext(ctx, r.runner, &dbr, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, organization.ErrRoleNotFound
		}
		return nil, err
	}
	return dbr.toDomain(), nil
}

func (r *OrganizationRepository) GetRoleByName(ctx context.Context, orgID uuid.UUID, name string) (*organization.Role, error) {
	query, args, _ := r.sq.Select("*").From("org_roles").
		Where(squirrel.Eq{"organization_id": orgID, "name": name}).ToSql()
	var dbr dbRole
	if err := sqlx.GetContext(ctx, r.runner, &dbr, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, organization.ErrRoleNotFound
		}
		return nil, err
	}
	return dbr.toDomain(), nil
}

func (r *OrganizationRepository) GetSystemRole(ctx context.Context, orgID uuid.UUID, name string) (*organization.Role, error) {
	query, args, _ := r.sq.Select("*").From("org_roles").
		Where(squirrel.Eq{"organization_id": orgID, "name": name, "is_system_role": true}).ToSql()
	var dbr dbRole
	if err := sqlx.GetContext(ctx, r.runner, &dbr, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, organization.ErrRoleNotFound
		}
		return nil, err
	}
	return dbr.toDomain(), nil
}

func (r *OrganizationRepository) ListRole(ctx context.Context, orgID uuid.UUID) ([]*organization.Role, error) {
	query, args, _ := r.sq.Select("*").From("org_roles").
		Where(squirrel.Eq{"organization_id": orgID}).
		OrderBy("name ASC").ToSql()
	var rows []dbRole
	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, err
	}
	roles := make([]*organization.Role, len(rows))
	for i, row := range rows {
		roles[i] = row.toDomain()
	}
	return roles, nil
}

func (r *OrganizationRepository) UpdateRole(ctx context.Context, role *organization.Role) error {
	role.UpdatedAt = time.Now().UTC()
	dbModel := dbRoleFromDomain(role)
	query, args, err := r.sq.Update("org_roles").
		Set("name", dbModel.Name).
		Set("description", dbModel.Description).
		Set("permissions", dbModel.Permissions).
		Set("updated_at", dbModel.UpdatedAt).
		Where(squirrel.Eq{"id": dbModel.ID}).
		ToSql()
	if err != nil {
		return err
	}
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return organization.ErrRoleAlreadyExists
		}
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return organization.ErrRoleNotFound
	}
	return nil
}

func (r *OrganizationRepository) DeleteRole(ctx context.Context, id uuid.UUID) error {
	query, args, _ := r.sq.Delete("org_roles").Where(squirrel.Eq{"id": id}).ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "foreign_key_violation" {
			return errors.New("cannot delete role assigned to members")
		}
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return organization.ErrRoleNotFound
	}
	return nil
}

// --- InvitationRepository Implementation ---

func (r *OrganizationRepository) CreateInvitation(ctx context.Context, inv *organization.Invitation) error {
	dbModel := dbInvitationFromDomain(inv)
	query, args, err := r.sq.Insert("org_invitations").
		Columns("id", "organization_id", "email", "role_id", "invited_by", "status", "token_hash", "expires_at", "created_at", "updated_at").
		Values(dbModel.ID, dbModel.OrganizationID, dbModel.Email, dbModel.RoleID, dbModel.InvitedBy, dbModel.Status, dbModel.TokenHash, dbModel.ExpiresAt, dbModel.CreatedAt, dbModel.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *OrganizationRepository) GetInvitationByID(ctx context.Context, id uuid.UUID) (*organization.Invitation, error) {
	query, args, _ := r.sq.Select("*").From("org_invitations").Where(squirrel.Eq{"id": id}).ToSql()
	var dbi dbInvitation
	if err := sqlx.GetContext(ctx, r.runner, &dbi, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, organization.ErrInvitationNotFound
		}
		return nil, err
	}
	return dbi.toDomain(), nil
}

func (r *OrganizationRepository) GetInvitationByTokenHash(ctx context.Context, tokenHash string) (*organization.Invitation, error) {
	query, args, _ := r.sq.Select("*").From("org_invitations").Where(squirrel.Eq{"token_hash": tokenHash}).ToSql()
	var dbi dbInvitation
	if err := sqlx.GetContext(ctx, r.runner, &dbi, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, organization.ErrInvitationNotFound
		}
		return nil, err
	}
	return dbi.toDomain(), nil
}

func (r *OrganizationRepository) GetPendingInvitationsByEmail(ctx context.Context, orgID uuid.UUID, email string) (*organization.Invitation, error) {
	query, args, _ := r.sq.Select("*").From("org_invitations").
		Where(squirrel.Eq{
			"organization_id": orgID,
			"email":           email,
			"status":          organization.InvitationStatusPending,
		}).ToSql()
	var dbi dbInvitation
	if err := sqlx.GetContext(ctx, r.runner, &dbi, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, organization.ErrInvitationNotFound
		}
		return nil, err
	}
	return dbi.toDomain(), nil
}

func (r *OrganizationRepository) ListPendingInvitationsWithDetails(ctx context.Context, orgID uuid.UUID) ([]*organization.InvitationDetails, error) {
	// Needs joins for details
	query, args, _ := r.sq.Select(
		"i.id", "i.organization_id", "i.email", "i.status", "i.expires_at",
		"u.id as inviter_id", "u.first_name as inviter_fn", "u.last_name as inviter_ln", "u.email as inviter_email",
		"r.id as role_id", "r.name as role_name",
	).
		From("org_invitations i").
		Join("users u ON i.invited_by = u.id").
		Join("org_roles r ON i.role_id = r.id").
		Where(squirrel.Eq{"i.organization_id": orgID, "i.status": organization.InvitationStatusPending}).
		ToSql()

	var rows []dbInvitationDetailRow
	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, err
	}

	details := make([]*organization.InvitationDetails, len(rows))
	for i, row := range rows {
		details[i] = row.toDomain()
	}
	return details, nil
}

func (r *OrganizationRepository) UpdateInvitation(ctx context.Context, inv *organization.Invitation) error {
	inv.UpdatedAt = time.Now().UTC()
	dbModel := dbInvitationFromDomain(inv)
	query, args, err := r.sq.Update("org_invitations").
		Set("status", dbModel.Status).
		Set("accepted_by", dbModel.AcceptedBy).
		Set("updated_at", dbModel.UpdatedAt).
		Where(squirrel.Eq{"id": dbModel.ID}).
		ToSql()
	if err != nil {
		return err
	}
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return organization.ErrInvitationNotFound
	}
	return nil
}

func (r *OrganizationRepository) GetInvitationWithDetails(ctx context.Context, id uuid.UUID) (*organization.InvitationDetails, error) {
	query, args, _ := r.sq.Select(
		"i.id", "i.organization_id", "i.email", "i.status", "i.expires_at",
		"u.id as inviter_id", "u.first_name as inviter_fn", "u.last_name as inviter_ln", "u.email as inviter_email",
		"r.id as role_id", "r.name as role_name",
		"o.id as org_id", "o.name as org_name", "o.slug as org_slug", "o.type as org_type",
	).
		From("org_invitations i").
		Join("users u ON i.invited_by = u.id").
		Join("org_roles r ON i.role_id = r.id").
		Join("organizations o ON i.organization_id = o.id").
		Where(squirrel.Eq{"i.id": id}).
		ToSql()

	var row dbInvitationDetailRow
	if err := sqlx.GetContext(ctx, r.runner, &row, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, organization.ErrInvitationNotFound
		}
		return nil, err
	}
	return row.toDomain(), nil
}

func (r *OrganizationRepository) ListPendingInvitationsByEmail(ctx context.Context, email string) ([]*organization.Invitation, error) {
	query, args, _ := r.sq.Select("*").From("org_invitations").
		Where(squirrel.Eq{"email": email, "status": organization.InvitationStatusPending}).ToSql()
	var rows []dbInvitation
	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, err
	}
	invs := make([]*organization.Invitation, len(rows))
	for i, r := range rows {
		invs[i] = r.toDomain()
	}
	return invs, nil
}

func (r *OrganizationRepository) ExpirePendingInvitations(ctx context.Context, now time.Time) (int64, error) {
	query, args, _ := r.sq.Update("org_invitations").
		Set("status", organization.InvitationStatusExpired).
		Set("updated_at", now).
		Where(squirrel.Eq{"status": organization.InvitationStatusPending}).
		Where(squirrel.Lt{"expires_at": now}).
		ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// --- DomainRepository Implementation ---

func (r *OrganizationRepository) CreateDomain(ctx context.Context, domain *organization.OrganizationDomain) error {
	dbModel := dbOrganizationDomainFromDomain(domain)
	query, args, err := r.sq.Insert("organization_domains").
		Columns("id", "organization_id", "domain", "is_primary", "verified_at", "created_at", "updated_at").
		Values(dbModel.ID, dbModel.OrganizationID, dbModel.Domain, dbModel.IsPrimary, dbModel.VerifiedAt, dbModel.CreatedAt, dbModel.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *OrganizationRepository) GetDomain(ctx context.Context, id uuid.UUID) (*organization.OrganizationDomain, error) {
	query, args, _ := r.sq.Select("*").From("organization_domains").Where(squirrel.Eq{"id": id}).ToSql()
	var dbd dbOrganizationDomain
	if err := sqlx.GetContext(ctx, r.runner, &dbd, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("domain not found")
		}
		return nil, err
	}
	return dbd.toDomain(), nil
}

func (r *OrganizationRepository) ListDomains(ctx context.Context, orgID uuid.UUID) ([]*organization.OrganizationDomain, error) {
	query, args, _ := r.sq.Select("*").From("organization_domains").Where(squirrel.Eq{"organization_id": orgID}).ToSql()
	var rows []dbOrganizationDomain
	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, err
	}
	domains := make([]*organization.OrganizationDomain, len(rows))
	for i, row := range rows {
		domains[i] = row.toDomain()
	}
	return domains, nil
}

func (r *OrganizationRepository) UpdateDomain(ctx context.Context, domain *organization.OrganizationDomain) error {
	dbModel := dbOrganizationDomainFromDomain(domain)
	query, args, err := r.sq.Update("organization_domains").
		Set("domain", dbModel.Domain).
		Set("is_primary", dbModel.IsPrimary).
		Set("verified_at", dbModel.VerifiedAt).
		Set("updated_at", dbModel.UpdatedAt).
		Where(squirrel.Eq{"id": dbModel.ID}).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *OrganizationRepository) DeleteDomain(ctx context.Context, id uuid.UUID) error {
	query, args, _ := r.sq.Delete("organization_domains").Where(squirrel.Eq{"id": id}).ToSql()
	_, err := r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *OrganizationRepository) GetPrimaryDomain(ctx context.Context, orgID uuid.UUID) (*organization.OrganizationDomain, error) {
	query, args, _ := r.sq.Select("*").From("organization_domains").Where(squirrel.Eq{"organization_id": orgID, "is_primary": true}).ToSql()
	var dbd dbOrganizationDomain
	if err := sqlx.GetContext(ctx, r.runner, &dbd, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("primary domain not found")
		}
		return nil, err
	}
	return dbd.toDomain(), nil
}

type dbOrganization struct {
	ID        uuid.UUID  `db:"id"`
	OwnerID   uuid.UUID  `db:"owner_id"`
	Name      string     `db:"name"`
	Slug      string     `db:"slug"`
	Type      string     `db:"type"`
	Status    string     `db:"status"`
	AvatarID  *uuid.UUID `db:"avatar_id"`
	Version   int        `db:"version"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
}

func dbOrganizationFromDomain(o *organization.Organization) *dbOrganization {
	return &dbOrganization{
		ID:        o.ID,
		OwnerID:   o.OwnerID,
		Name:      o.Name,
		Slug:      o.Slug,
		Type:      string(o.Type),
		Status:    string(o.Status),
		AvatarID:  o.AvatarID,
		Version:   o.Version,
		CreatedAt: o.CreatedAt,
		UpdatedAt: o.UpdatedAt,
	}
}

func (d *dbOrganization) toDomain() *organization.Organization {
	return &organization.Organization{
		ID:        d.ID,
		OwnerID:   d.OwnerID,
		Name:      d.Name,
		Slug:      d.Slug,
		Type:      organization.OrganizationType(d.Type),
		Status:    organization.OrganizationStatus(d.Status),
		AvatarID:  d.AvatarID,
		Version:   d.Version,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}

type dbOrganizationDomain struct {
	ID             uuid.UUID  `db:"id"`
	OrganizationID uuid.UUID  `db:"organization_id"`
	Domain         string     `db:"domain"`
	IsPrimary      bool       `db:"is_primary"`
	VerifiedAt     *time.Time `db:"verified_at"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
}

func dbOrganizationDomainFromDomain(d *organization.OrganizationDomain) *dbOrganizationDomain {
	return &dbOrganizationDomain{
		ID:             d.ID,
		OrganizationID: d.OrganizationID,
		Domain:         d.Domain,
		IsPrimary:      d.IsPrimary,
		VerifiedAt:     d.VerifiedAt,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}
}

func (d *dbOrganizationDomain) toDomain() *organization.OrganizationDomain {
	return &organization.OrganizationDomain{
		ID:             d.ID,
		OrganizationID: d.OrganizationID,
		Domain:         d.Domain,
		IsPrimary:      d.IsPrimary,
		VerifiedAt:     d.VerifiedAt,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}
}

type dbRole struct {
	ID             uuid.UUID      `db:"id"`
	OrganizationID uuid.UUID      `db:"organization_id"`
	Name           string         `db:"name"`
	Description    string         `db:"description"`
	IsSystemRole   bool           `db:"is_system_role"`
	Permissions    pq.StringArray `db:"permissions"` // Handles postgres text[]
	CreatedAt      time.Time      `db:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at"`
}

func dbRoleFromDomain(r *organization.Role) *dbRole {
	return &dbRole{
		ID:             r.ID,
		OrganizationID: r.OrganizationID,
		Name:           r.Name,
		Description:    r.Description,
		IsSystemRole:   r.IsSystemRole,
		Permissions:    pq.StringArray(r.Permissions),
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

func (d *dbRole) toDomain() *organization.Role {
	return &organization.Role{
		ID:             d.ID,
		OrganizationID: d.OrganizationID,
		Name:           d.Name,
		Description:    d.Description,
		IsSystemRole:   d.IsSystemRole,
		Permissions:    []string(d.Permissions),
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}
}

type dbMember struct {
	OrganizationID uuid.UUID `db:"organization_id"`
	UserID         uuid.UUID `db:"user_id"`
	RoleID         uuid.UUID `db:"role_id"`
	JoinedAt       time.Time `db:"joined_at"`
}

func (d *dbMember) toDomain() *organization.Member {
	return &organization.Member{
		OrganizationID: d.OrganizationID,
		UserID:         d.UserID,
		RoleID:         d.RoleID,
		JoinedAt:       d.JoinedAt,
	}
}

type dbInvitation struct {
	ID             uuid.UUID  `db:"id"`
	OrganizationID uuid.UUID  `db:"organization_id"`
	Email          string     `db:"email"`
	RoleID         uuid.UUID  `db:"role_id"`
	InvitedBy      uuid.UUID  `db:"invited_by"`
	AcceptedBy     *uuid.UUID `db:"accepted_by"`
	Status         string     `db:"status"`
	TokenHash      string     `db:"token_hash"`
	ExpiresAt      time.Time  `db:"expires_at"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
}

func dbInvitationFromDomain(i *organization.Invitation) *dbInvitation {
	return &dbInvitation{
		ID:             i.ID,
		OrganizationID: i.OrganizationID,
		Email:          i.Email,
		RoleID:         i.RoleID,
		InvitedBy:      i.InvitedBy,
		AcceptedBy:     i.AcceptedBy,
		Status:         string(i.Status),
		TokenHash:      i.TokenHash,
		ExpiresAt:      i.ExpiresAt,
		CreatedAt:      i.CreatedAt,
		UpdatedAt:      i.UpdatedAt,
	}
}

func (d *dbInvitation) toDomain() *organization.Invitation {
	return &organization.Invitation{
		ID:             d.ID,
		OrganizationID: d.OrganizationID,
		Email:          d.Email,
		RoleID:         d.RoleID,
		InvitedBy:      d.InvitedBy,
		AcceptedBy:     d.AcceptedBy,
		Status:         organization.InvitationStatus(d.Status),
		TokenHash:      d.TokenHash,
		ExpiresAt:      d.ExpiresAt,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}
}

type dbInvitationDetailRow struct {
	ID             uuid.UUID      `db:"id"`
	OrganizationID uuid.UUID      `db:"organization_id"`
	Email          string         `db:"email"`
	Status         string         `db:"status"`
	ExpiresAt      time.Time      `db:"expires_at"`
	InviterID      uuid.UUID      `db:"inviter_id"`
	InviterFN      string         `db:"inviter_fn"`
	InviterLN      string         `db:"inviter_ln"`
	InviterEmail   string         `db:"inviter_email"`
	RoleID         uuid.UUID      `db:"role_id"`
	RoleName       string         `db:"role_name"`
	OrgID          sql.NullString `db:"org_id"`   // Nullable left join artifact if unused?
	OrgName        sql.NullString `db:"org_name"` // Used for GetInvitationWithDetails
	OrgSlug        sql.NullString `db:"org_slug"`
	OrgType        sql.NullString `db:"org_type"`
}

func (d *dbInvitationDetailRow) toDomain() *organization.InvitationDetails {
	inviterEmail, _ := core.NewEmail(d.InviterEmail) // Handle error?

	details := &organization.InvitationDetails{
		ID:             d.ID,
		OrganizationID: d.OrganizationID,
		Email:          d.Email,
		Status:         organization.InvitationStatus(d.Status),
		ExpiresAt:      d.ExpiresAt,
		Inviter: &user.User{
			ID:        d.InviterID,
			FirstName: d.InviterFN,
			LastName:  d.InviterLN,
			Email:     inviterEmail,
		},
		Role: &organization.Role{
			ID:   d.RoleID,
			Name: d.RoleName,
		},
	}

	if d.OrgID.Valid {
		id, _ := uuid.Parse(d.OrgID.String)
		details.Organization = &organization.Organization{
			ID:   id,
			Name: d.OrgName.String,
			Slug: d.OrgSlug.String,
			Type: organization.OrganizationType(d.OrgType.String),
		}
	}

	return details
}

// Use explicit column aliases in SQL to match these tags
type dbMemberDetailsRow struct {
	JoinedAt time.Time `db:"joined_at"`

	// User Prefix: u_
	UserID        uuid.UUID `db:"u_id"`
	UserFirstName string    `db:"u_first_name"`
	UserLastName  string    `db:"u_last_name"`
	UserEmail     string    `db:"u_email"`
	UserStatus    string    `db:"u_status"`
	UserCreatedAt time.Time `db:"u_created_at"`
	UserUpdatedAt time.Time `db:"u_updated_at"`

	// Role Prefix: r_
	RoleID          uuid.UUID      `db:"r_id"`
	RoleOrgID       uuid.UUID      `db:"r_organization_id"`
	RoleName        string         `db:"r_name"`
	RoleDescription string         `db:"r_description"`
	RoleIsSystem    bool           `db:"r_is_system_role"`
	RolePermissions pq.StringArray `db:"r_permissions"`
	RoleCreatedAt   time.Time      `db:"r_created_at"`
	RoleUpdatedAt   time.Time      `db:"r_updated_at"`
}

func (d *dbMemberDetailsRow) toDomain() (*organization.MemberDetails, error) {
	email, err := core.NewEmail(d.UserEmail)
	if err != nil {
		return nil, err
	}

	return &organization.MemberDetails{
		UserID:   d.UserID,
		JoinedAt: d.JoinedAt,
		User: &user.User{
			ID:        d.UserID,
			FirstName: d.UserFirstName,
			LastName:  d.UserLastName,
			Email:     email,
			Status:    user.UserStatus(d.UserStatus),
			CreatedAt: d.UserCreatedAt,
			UpdatedAt: d.UserUpdatedAt,
		},
		Role: &organization.Role{
			ID:             d.RoleID,
			OrganizationID: d.RoleOrgID,
			Name:           d.RoleName,
			Description:    d.RoleDescription,
			IsSystemRole:   d.RoleIsSystem,
			Permissions:    []string(d.RolePermissions),
			CreatedAt:      d.RoleCreatedAt,
			UpdatedAt:      d.RoleUpdatedAt,
		},
	}, nil
}
