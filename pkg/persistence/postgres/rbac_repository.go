package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/apperror"
	"github.com/combi/pkg/log"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var _ rbac.Repository = (*RBACRepository)(nil)

type RBACRepository struct {
	db     *sqlx.DB
	runner sqlx.ExtContext
	logger log.Logger
	sq     squirrel.StatementBuilderType
}

func NewRBACRepository(db *sqlx.DB, logger log.Logger) *RBACRepository {
	return &RBACRepository{
		db:     db,
		runner: db,
		logger: logger.With(log.String("component", "rbac_repository")),
		sq:     squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

func (r *RBACRepository) WithTx(ctx context.Context, fn func(txRepo rbac.Repository) error) (err error) {
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
			if rbErr := tx.Rollback(); rbErr != nil {
				r.logger.Error("transaction rollback failed", rbErr, log.String("original_error", err.Error()))
			}
		} else {
			err = tx.Commit()
		}
	}()
	txRepo := &RBACRepository{
		db:     r.db,
		runner: tx,
		logger: r.logger,
		sq:     squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
	return fn(txRepo)
}

// --- PermissionRepository ---

func (r *RBACRepository) UpsertPermissions(ctx context.Context, permissions []*rbac.Permission) error {
	if len(permissions) == 0 {
		return nil
	}
	builder := r.sq.Insert("permissions").Columns("id", "name", "description", "category", "is_system", "created_at", "updated_at")
	for _, p := range permissions {
		builder = builder.Values(p.ID, p.Name, p.Description, p.Category, p.IsSystem, p.CreatedAt, p.UpdatedAt)
	}
	query, args, err := builder.Suffix(
		"ON CONFLICT (name) DO UPDATE SET description = EXCLUDED.description, category = EXCLUDED.category, is_system = EXCLUDED.is_system, updated_at = NOW() AT TIME ZONE 'utc'").
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to build upsert permissions query: %w", err)
	}
	_, err = r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *RBACRepository) GetPermissionByName(ctx context.Context, name core.PermissionName) (*rbac.Permission, error) {
	query, args, _ := r.sq.Select("*").From("permissions").Where(squirrel.Eq{"name": name}).ToSql()
	var perm rbac.Permission
	if err := sqlx.GetContext(ctx, r.runner, &perm, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, rbac.ErrPermissionNotFound
		}
		return nil, err
	}
	return &perm, nil
}

func (r *RBACRepository) ListPermissions(ctx context.Context, filters rbac.PermissionFilters) ([]*rbac.Permission, int, error) {
	builder := r.sq.Select("*, COUNT(*) OVER() as total_count").From("permissions")
	if filters.Category != nil {
		builder = builder.Where(squirrel.Eq{"category": *filters.Category})
	}
	if filters.Search != "" {
		builder = builder.Where(squirrel.ILike{"name": fmt.Sprintf("%%%s%%", filters.Search)})
	}
	query, args, err := builder.OrderBy("category, name").Limit(uint64(filters.Limit)).Offset(uint64(filters.Offset)).ToSql()
	if err != nil {
		return nil, 0, err
	}
	var results []dbPermissionWithTotal
	if err := sqlx.SelectContext(ctx, r.runner, &results, query, args...); err != nil {
		return nil, 0, err
	}
	if len(results) == 0 {
		return []*rbac.Permission{}, 0, nil
	}
	total := results[0].Total
	permissions := make([]*rbac.Permission, len(results))
	for i := range results {
		permissions[i] = &results[i].Permission
	}
	return permissions, total, nil
}

func (r *RBACRepository) GetPermissionsByNames(ctx context.Context, names []core.PermissionName) ([]*rbac.Permission, error) {
	if len(names) == 0 {
		return []*rbac.Permission{}, nil
	}
	query, args, _ := r.sq.Select("*").From("permissions").Where(squirrel.Eq{"name": names}).ToSql()
	var perms []rbac.Permission
	if err := sqlx.SelectContext(ctx, r.runner, &perms, query, args...); err != nil {
		return nil, err
	}
	result := make([]*rbac.Permission, len(perms))
	for i := range perms {
		result[i] = &perms[i]
	}
	return result, nil
}

func (r *RBACRepository) UpdatePermission(ctx context.Context, perm *rbac.Permission) error {
	perm.UpdatedAt = time.Now().UTC()
	query, args, err := r.sq.Update("permissions").
		Set("description", perm.Description).
		Set("category", perm.Category).
		Set("is_system", perm.IsSystem).
		Set("updated_at", perm.UpdatedAt).
		Where(squirrel.Eq{"id": perm.ID}).
		ToSql()
	if err != nil {
		return err
	}
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return rbac.ErrPermissionNotFound
	}
	return nil
}

// --- RoleRepository ---

func (r *RBACRepository) CreateRole(ctx context.Context, role *rbac.Role) error {
	query, args, err := r.sq.Insert("roles").SetMap(roleToMap(role)).ToSql()
	if err != nil {
		return err
	}
	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			switch pqErr.Code.Name() {
			case "unique_violation":
				return rbac.ErrRoleAlreadyExists
			case "string_data_right_truncation":
				return apperror.New(http.StatusBadRequest, "INVALID_INPUT", "Field too long", err)
			}
		}
		return err
	}
	return nil
}
func (r *RBACRepository) getRole(ctx context.Context, field string, value any) (*rbac.Role, error) {
	query, args, _ := r.sq.Select("*").From("roles").Where(squirrel.Eq{field: value}).ToSql()
	var role rbac.Role
	if err := sqlx.GetContext(ctx, r.runner, &role, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, rbac.ErrRoleNotFound
		}
		return nil, err
	}
	return &role, nil
}

func (r *RBACRepository) GetRole(ctx context.Context, id uuid.UUID) (*rbac.Role, error) {
	return r.getRole(ctx, "id", id)
}

func (r *RBACRepository) GetRoleBySlug(ctx context.Context, slug string) (*rbac.Role, error) {
	return r.getRole(ctx, "slug", slug)
}

func (r *RBACRepository) ListRoles(ctx context.Context, filters rbac.RoleFilters) ([]*rbac.Role, int, error) {
	builder := r.sq.Select("*, COUNT(*) OVER() as total_count").From("roles")
	query, args, err := builder.OrderBy("name").Limit(uint64(filters.Limit)).Offset(uint64(filters.Offset)).ToSql()
	if err != nil {
		return nil, 0, err
	}
	var results []dbRoleWithTotal
	if err := sqlx.SelectContext(ctx, r.runner, &results, query, args...); err != nil {
		return nil, 0, err
	}
	if len(results) == 0 {
		return []*rbac.Role{}, 0, nil
	}
	total := results[0].Total
	roles := make([]*rbac.Role, len(results))
	for i := range results {
		roles[i] = &results[i].Role
	}
	return roles, total, nil
}

func (r *RBACRepository) UpdateRole(ctx context.Context, role *rbac.Role) error {
	role.UpdatedAt = time.Now().UTC()
	query, args, err := r.sq.Update("roles").SetMap(roleToMap(role)).Where(squirrel.Eq{"id": role.ID}).ToSql()
	if err != nil {
		return err
	}
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return rbac.ErrRoleAlreadyExists
		}
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return rbac.ErrRoleNotFound
	}
	return nil
}

func (r *RBACRepository) DeleteRole(ctx context.Context, id uuid.UUID) error {
	query, args, _ := r.sq.Delete("roles").Where(squirrel.Eq{"id": id}).ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "foreign_key_violation" {
			return rbac.ErrRoleInUse
		}
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return rbac.ErrRoleNotFound
	}
	return nil
}

func (r *RBACRepository) AddPermissionsToRole(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) error {
	if len(permissionIDs) == 0 {
		return nil
	}
	builder := r.sq.Insert("role_permissions").Columns("role_id", "permission_id")
	for _, permID := range permissionIDs {
		builder = builder.Values(roleID, permID)
	}
	query, args, _ := builder.Suffix("ON CONFLICT DO NOTHING").ToSql()
	_, err := r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *RBACRepository) RemovePermissionsFromRole(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) error {
	if len(permissionIDs) == 0 {
		return nil
	}
	query, args, _ := r.sq.Delete("role_permissions").
		Where(squirrel.Eq{"role_id": roleID, "permission_id": permissionIDs}).
		ToSql()
	_, err := r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *RBACRepository) GetRolePermissionNames(ctx context.Context, roleID uuid.UUID) ([]core.PermissionName, error) {
	query, args, _ := r.sq.Select("p.name").From("permissions p").
		Join("role_permissions rp ON p.id = rp.permission_id").
		Where(squirrel.Eq{"rp.role_id": roleID}).
		ToSql()
	var names []core.PermissionName
	if err := sqlx.SelectContext(ctx, r.runner, &names, query, args...); err != nil {
		return nil, err
	}
	return names, nil
}

// --- RoleAssignmentRepository ---

func (r *RBACRepository) AssignRoleToUser(ctx context.Context, assignment *rbac.RoleAssignment) error {
	query, args, _ := r.sq.Insert("user_roles").
		Columns("id", "user_id", "role_id", "granted_by", "expires_at", "created_at").
		Values(assignment.ID, assignment.UserID, assignment.RoleID, assignment.GrantedBy, assignment.ExpiresAt, assignment.CreatedAt).
		Suffix("ON CONFLICT (user_id, role_id) DO NOTHING").
		ToSql()
	_, err := r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *RBACRepository) RevokeRoleFromUser(ctx context.Context, userID, roleID uuid.UUID) error {
	query, args, _ := r.sq.Delete("user_roles").
		Where(squirrel.Eq{"user_id": userID, "role_id": roleID}).
		ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return rbac.ErrRoleAssignmentNotFound
	}
	return nil
}

func (r *RBACRepository) GetUserRoleIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	query, args, _ := r.sq.Select("role_id").From("user_roles").Where(squirrel.Eq{"user_id": userID}).ToSql()
	var ids []uuid.UUID
	if err := sqlx.SelectContext(ctx, r.runner, &ids, query, args...); err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *RBACRepository) GetUserRoles(ctx context.Context, userID uuid.UUID) ([]*rbac.Role, error) {
	query, args, _ := r.sq.Select("r.*").From("roles r").
		Join("user_roles ur ON r.id = ur.role_id").
		Where(squirrel.Eq{"ur.user_id": userID}).
		ToSql()
	var roles []rbac.Role
	if err := sqlx.SelectContext(ctx, r.runner, &roles, query, args...); err != nil {
		return nil, err
	}
	result := make([]*rbac.Role, len(roles))
	for i := range roles {
		result[i] = &roles[i]
	}
	return result, nil
}

func (r *RBACRepository) CountUsersInRole(ctx context.Context, roleID uuid.UUID) (int, error) {
	query, args, _ := r.sq.Select("COUNT(*)").From("user_roles").Where(squirrel.Eq{"role_id": roleID}).ToSql()
	var count int
	if err := sqlx.GetContext(ctx, r.runner, &count, query, args...); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *RBACRepository) GetUserEffectivePermissionNames(ctx context.Context, userID uuid.UUID) ([]core.PermissionName, error) {
	query, args, _ := r.sq.Select("DISTINCT p.name").
		From("permissions p").
		Join("role_permissions rp ON p.id = rp.permission_id").
		Join("user_roles ur ON rp.role_id = ur.role_id").
		Join("roles r ON ur.role_id = r.id").
		Where(squirrel.Eq{"ur.user_id": userID, "r.is_active": true}).
		ToSql()

	var names []core.PermissionName
	if err := sqlx.SelectContext(ctx, r.runner, &names, query, args...); err != nil {
		return nil, err
	}
	return names, nil
}

func (r *RBACRepository) ListUsersByRoleSlug(ctx context.Context, roleSlug string) ([]rbac.UserRoleSummary, error) {
	// 1. The SQL Query: Joins users, user_roles, and roles tables
	query, args, _ := r.sq.Select(
		"u.id",
		"u.first_name",
		"u.last_name",
		"u.email",
		"ur.created_at", // When they were assigned the role
	).
		From("users u").
		Join("user_roles ur ON u.id = ur.user_id").
		Join("roles r ON ur.role_id = r.id").
		Where(squirrel.Eq{
			"r.slug":      roleSlug,
			"r.is_active": true,
		}).
		OrderBy("u.last_name ASC, u.first_name ASC").
		ToSql()

	// 2. The Mapping: SQL rows -> UserRoleSummary struct
	var summaries []rbac.UserRoleSummary
	if err := sqlx.SelectContext(ctx, r.runner, &summaries, query, args...); err != nil {
		return nil, err
	}

	return summaries, nil
}

// --- Helper Structs ---

type dbPermissionWithTotal struct {
	rbac.Permission `db:",squash"`
	Total           int `db:"total_count"`
}
type dbRoleWithTotal struct {
	rbac.Role `db:",squash"`
	Total     int `db:"total_count"`
}

func roleToMap(role *rbac.Role) map[string]interface{} {
	return map[string]interface{}{
		"id":          role.ID,
		"name":        role.Name,
		"slug":        role.Slug,
		"description": role.Description,
		"type":        role.Type,
		"is_active":   role.IsActive,
		"created_by":  role.CreatedBy,
		"created_at":  role.CreatedAt,
		"updated_at":  role.UpdatedAt,
	}
}

// type dbUser struct {
// 	ID        uuid.UUID `db:"id"`
// 	FirstName string    `db:"first_name"`
// 	LastName  string    `db:"last_name"`
// 	Email     string    `db:"email"`
// 	Status    string    `db:"status"`
// 	CreatedAt time.Time `db:"created_at"`
// 	UpdatedAt time.Time `db:"updated_at"`
// }

// func (d *dbUser) toDomain() (*user.User, error) {
// 	email, err := core.NewEmail(d.Email)
// 	if err != nil {
// 		return nil, fmt.Errorf("invalid email in database for user %s: %w", d.ID, err)
// 	}
// 	return &user.User{
// 		ID:        d.ID,
// 		FirstName: d.FirstName,
// 		LastName:  d.LastName,
// 		Email:     email,
// 		Status:    user.UserStatus(d.Status),
// 		CreatedAt: d.CreatedAt,
// 		UpdatedAt: d.UpdatedAt,
// 	}, nil
// }
