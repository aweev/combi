package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/log"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var _ user.Repository = (*UserRepository)(nil)

type UserRepository struct {
	db     *sqlx.DB
	runner sqlx.ExtContext
	logger log.Logger
	sq     squirrel.StatementBuilderType
}

func NewUserRepository(db *sqlx.DB, logger log.Logger) *UserRepository {
	return &UserRepository{
		db:     db,
		runner: db,
		logger: logger.With(log.String("component", "user_repository")),
		sq:     squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

func (r *UserRepository) WithTx(ctx context.Context, fn func(txRepo user.Repository) error) (err error) {
	if _, ok := r.runner.(*sqlx.Tx); ok {
		return fn(r)
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	txRepo := &UserRepository{
		db:     r.db,
		runner: tx,
		logger: r.logger,
		sq:     r.sq,
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(txRepo); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

// --- UserRepository Implementation ---

func (r *UserRepository) CreateUser(ctx context.Context, u *user.User) error {
	dbModel := dbUserFromDomain(u)
	query, args, err := r.sq.Insert("users").
		Columns("id", "first_name", "last_name", "email", "status", "locale", "created_at", "updated_at").
		Values(dbModel.ID, dbModel.FirstName, dbModel.LastName, dbModel.Email, dbModel.Status, dbModel.Locale, dbModel.CreatedAt, dbModel.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return fmt.Errorf("user already exists: %w", err)
		}
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

func (r *UserRepository) GetUser(ctx context.Context, userID uuid.UUID) (*user.User, error) {
	return r.getUser(ctx, squirrel.Eq{"id": userID})
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*user.User, error) {
	return r.getUser(ctx, squirrel.Eq{"email": email})
}

func (r *UserRepository) getUser(ctx context.Context, pred interface{}) (*user.User, error) {
	query, args, _ := r.sq.Select("*").From("users").Where(pred).ToSql()
	var dbu dbUser
	if err := sqlx.GetContext(ctx, r.runner, &dbu, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, user.ErrProfileNotFound
		}
		return nil, err
	}
	return dbu.toDomain()
}

func (r *UserRepository) ListUsers(ctx context.Context, filter user.ListUsersFilter) ([]*user.User, int, error) {
	builder := r.sq.Select("*, COUNT(*) OVER() as total_count").From("users")

	if filter.Status != nil {
		builder = builder.Where(squirrel.Eq{"status": *filter.Status})
	}
	if filter.Search != "" {
		pattern := "%" + filter.Search + "%"
		builder = builder.Where(squirrel.Or{
			squirrel.ILike{"email": pattern},
			squirrel.ILike{"first_name": pattern},
			squirrel.ILike{"last_name": pattern},
		})
	}

	query, args, err := builder.
		OrderBy("created_at DESC").
		Limit(uint64(filter.Limit)).
		Offset(uint64(filter.Offset)).
		ToSql()
	if err != nil {
		return nil, 0, err
	}

	var results []dbUserWithTotal
	if err := sqlx.SelectContext(ctx, r.runner, &results, query, args...); err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return []*user.User{}, 0, nil
	}

	users := make([]*user.User, len(results))
	for i, res := range results {
		u, err := res.toDomain()
		if err != nil {
			return nil, 0, err
		}
		users[i] = u
	}

	return users, results[0].Total, nil
}

func (r *UserRepository) UpdateUser(ctx context.Context, u *user.User) error {
	u.UpdatedAt = time.Now().UTC()
	dbModel := dbUserFromDomain(u)

	query, args, err := r.sq.Update("users").
		Set("first_name", dbModel.FirstName).
		Set("last_name", dbModel.LastName).
		Set("email", dbModel.Email).
		Set("status", dbModel.Status).
		Set("locale", dbModel.Locale).
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
		return user.ErrProfileNotFound
	}
	return nil
}

func (r *UserRepository) AnonymizeUser(ctx context.Context, userID uuid.UUID) error {
	scrambled := "anonymized-" + uuid.NewString()[:8]
	query, args, _ := r.sq.Update("users").
		Set("first_name", "Deleted").
		Set("last_name", "User").
		Set("email", scrambled+"@deleted.local").
		Set("status", user.StatusAnonymized).
		Set("updated_at", time.Now().UTC()).
		Where(squirrel.Eq{"id": userID}).
		ToSql()

	_, err := r.runner.ExecContext(ctx, query, args...)
	return err
}

// NEW: DeletePendingUsers removes users in PENDING status older than a specific time.
func (r *UserRepository) DeletePendingUsers(ctx context.Context, olderThan time.Time) (int64, error) {
	query, args, _ := r.sq.Delete("users").
		Where(squirrel.Eq{"status": user.StatusPending}).
		Where(squirrel.Lt{"created_at": olderThan}).
		ToSql()

	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to delete pending users: %w", err)
	}
	return res.RowsAffected()
}

func (r *UserRepository) GetPublicProfiles(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]user.PublicProfile, error) {
	if len(userIDs) == 0 {
		return map[uuid.UUID]user.PublicProfile{}, nil
	}

	query, args, _ := r.sq.Select(
		"u.id as user_id",
		"u.first_name",
		"u.last_name",
		"u.email",
		"p.display_name",
		"p.avatar_file_id",
	).
		From("users u").
		LeftJoin("profiles p ON u.id = p.id").
		Where(squirrel.Eq{"u.id": userIDs}).
		ToSql()

	var results []dbPublicProfile
	if err := sqlx.SelectContext(ctx, r.runner, &results, query, args...); err != nil {
		return nil, err
	}

	resultMap := make(map[uuid.UUID]user.PublicProfile)
	for _, res := range results {
		resultMap[res.UserID] = res.toDomain()
	}
	return resultMap, nil
}

// --- ProfileRepository Implementation ---

func (r *UserRepository) CreateProfile(ctx context.Context, p *user.Profile) error {
	dbModel, err := dbProfileFromDomain(p)
	if err != nil {
		return err
	}

	query, args, err := r.sq.Insert("profiles").
		Columns("id", "handle", "display_name", "avatar_file_id", "preferences", "created_at", "updated_at").
		Values(dbModel.ID, dbModel.Handle, dbModel.DisplayName, dbModel.AvatarFileID, dbModel.Preferences, dbModel.CreatedAt, dbModel.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return user.ErrHandleTaken
		}
		return fmt.Errorf("failed to create profile: %w", err)
	}
	return nil
}

func (r *UserRepository) GetProfile(ctx context.Context, userID uuid.UUID) (*user.Profile, error) {
	query, args, _ := r.sq.Select("*").From("profiles").Where(squirrel.Eq{"id": userID}).ToSql()

	var dbp dbProfile
	if err := sqlx.GetContext(ctx, r.runner, &dbp, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, user.ErrProfileNotFound
		}
		return nil, err
	}
	return dbp.toDomain()
}

func (r *UserRepository) UpdateProfile(ctx context.Context, p *user.Profile) error {
	p.UpdatedAt = time.Now().UTC()
	dbModel, err := dbProfileFromDomain(p)
	if err != nil {
		return err
	}

	query, args, err := r.sq.Update("profiles").
		Set("handle", dbModel.Handle).
		Set("display_name", dbModel.DisplayName).
		Set("avatar_file_id", dbModel.AvatarFileID).
		Set("preferences", dbModel.Preferences).
		Set("updated_at", dbModel.UpdatedAt).
		Where(squirrel.Eq{"id": dbModel.ID}).
		ToSql()
	if err != nil {
		return err
	}

	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return user.ErrHandleTaken
		}
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return user.ErrProfileNotFound
	}
	return nil
}

// --- ConnectionRepository Implementation ---

func (r *UserRepository) CreateConnection(ctx context.Context, conn *user.Connection) error {
	query, args, _ := r.sq.Insert("connections").
		Columns("id", "source_user_id", "target_user_id", "type", "created_at").
		Values(conn.ID, conn.SourceUserID, conn.TargetUserID, conn.Type, conn.CreatedAt).
		Suffix("ON CONFLICT (source_user_id, target_user_id, type) DO NOTHING").
		ToSql()

	_, err := r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *UserRepository) GetConnection(ctx context.Context, sourceID, targetID uuid.UUID, connType user.ConnectionType) (*user.Connection, error) {
	query, args, _ := r.sq.Select("*").From("connections").
		Where(squirrel.Eq{
			"source_user_id": sourceID,
			"target_user_id": targetID,
			"type":           connType,
		}).ToSql()

	var dbc dbConnection
	if err := sqlx.GetContext(ctx, r.runner, &dbc, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("connection not found")
		}
		return nil, err
	}
	return dbc.toDomain(), nil
}

func (r *UserRepository) DeleteConnection(ctx context.Context, id uuid.UUID) error {
	query, args, _ := r.sq.Delete("connections").Where(squirrel.Eq{"id": id}).ToSql()
	_, err := r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *UserRepository) ListConnectionTargetIDs(ctx context.Context, sourceID uuid.UUID, connType user.ConnectionType) ([]uuid.UUID, error) {
	query, args, _ := r.sq.Select("target_user_id").From("connections").
		Where(squirrel.Eq{
			"source_user_id": sourceID,
			"type":           connType,
		}).ToSql()

	var ids []uuid.UUID
	if err := sqlx.SelectContext(ctx, r.runner, &ids, query, args...); err != nil {
		return nil, err
	}
	return ids, nil
}

// NEW: ListConnectionSourceIDs implementation
func (r *UserRepository) ListConnectionSourceIDs(ctx context.Context, targetID uuid.UUID, connType user.ConnectionType) ([]uuid.UUID, error) {
	query, args, _ := r.sq.Select("source_user_id").From("connections").
		Where(squirrel.Eq{
			"target_user_id": targetID,
			"type":           connType,
		}).ToSql()

	var ids []uuid.UUID
	if err := sqlx.SelectContext(ctx, r.runner, &ids, query, args...); err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *UserRepository) ListConnections(ctx context.Context, sourceID uuid.UUID, connType user.ConnectionType) ([]*user.Connection, error) {
	query, args, _ := r.sq.Select("*").From("connections").
		Where(squirrel.Eq{
			"source_user_id": sourceID,
			"type":           connType,
		}).ToSql()

	var dbcs []dbConnection
	if err := sqlx.SelectContext(ctx, r.runner, &dbcs, query, args...); err != nil {
		return nil, err
	}

	result := make([]*user.Connection, len(dbcs))
	for i, dbc := range dbcs {
		result[i] = dbc.toDomain()
	}
	return result, nil
}

// --- DB Models & Mappers ---

type dbUser struct {
	ID        uuid.UUID      `db:"id"`
	FirstName string         `db:"first_name"`
	LastName  string         `db:"last_name"`
	Email     string         `db:"email"`
	Status    string         `db:"status"`
	Locale    sql.NullString `db:"locale"`
	CreatedAt time.Time      `db:"created_at"`
	UpdatedAt time.Time      `db:"updated_at"`
}

type dbUserWithTotal struct {
	dbUser
	Total int `db:"total_count"`
}

func dbUserFromDomain(u *user.User) *dbUser {
	return &dbUser{
		ID:        u.ID,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Email:     u.Email.String(),
		Status:    string(u.Status),
		Locale:    toNullString(u.Locale),
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

func (d *dbUser) toDomain() (*user.User, error) {
	email, err := core.NewEmail(d.Email)
	if err != nil {
		return nil, err
	}

	var locale *string
	if d.Locale.Valid {
		locale = &d.Locale.String
	}

	return &user.User{
		ID:        d.ID,
		FirstName: d.FirstName,
		LastName:  d.LastName,
		Email:     email,
		Status:    user.UserStatus(d.Status),
		Locale:    locale,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}, nil
}

type dbProfile struct {
	ID           uuid.UUID     `db:"id"`
	Handle       string        `db:"handle"`
	DisplayName  string        `db:"display_name"`
	AvatarFileID uuid.NullUUID `db:"avatar_file_id"`
	Preferences  []byte        `db:"preferences"`
	CreatedAt    time.Time     `db:"created_at"`
	UpdatedAt    time.Time     `db:"updated_at"`
}

func dbProfileFromDomain(p *user.Profile) (*dbProfile, error) {
	prefBytes, err := json.Marshal(p.Preferences)
	if err != nil {
		return nil, err
	}

	var avatar uuid.NullUUID
	if p.AvatarFileID != nil {
		avatar.UUID = *p.AvatarFileID
		avatar.Valid = true
	}

	return &dbProfile{
		ID:           p.ID,
		Handle:       p.Handle,
		DisplayName:  p.DisplayName,
		AvatarFileID: avatar,
		Preferences:  prefBytes,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}, nil
}

func (d *dbProfile) toDomain() (*user.Profile, error) {
	var prefs user.UserPreferences
	if len(d.Preferences) > 0 {
		if err := json.Unmarshal(d.Preferences, &prefs); err != nil {
			return nil, err
		}
	}

	var avatar *uuid.UUID
	if d.AvatarFileID.Valid {
		avatar = &d.AvatarFileID.UUID
	}

	return &user.Profile{
		ID:           d.ID,
		Handle:       d.Handle,
		DisplayName:  d.DisplayName,
		AvatarFileID: avatar,
		Preferences:  prefs,
		CreatedAt:    d.CreatedAt,
		UpdatedAt:    d.UpdatedAt,
	}, nil
}

type dbConnection struct {
	ID           uuid.UUID `db:"id"`
	SourceUserID uuid.UUID `db:"source_user_id"`
	TargetUserID uuid.UUID `db:"target_user_id"`
	Type         string    `db:"type"`
	CreatedAt    time.Time `db:"created_at"`
}

func (d *dbConnection) toDomain() *user.Connection {
	return &user.Connection{
		ID:           d.ID,
		SourceUserID: d.SourceUserID,
		TargetUserID: d.TargetUserID,
		Type:         user.ConnectionType(d.Type),
		CreatedAt:    d.CreatedAt,
	}
}

type dbPublicProfile struct {
	UserID       uuid.UUID      `db:"user_id"`
	FirstName    string         `db:"first_name"`
	LastName     string         `db:"last_name"`
	Email        string         `db:"email"`
	DisplayName  sql.NullString `db:"display_name"`
	AvatarFileID uuid.NullUUID  `db:"avatar_file_id"`
}

func (d *dbPublicProfile) toDomain() user.PublicProfile {
	var disp *string
	if d.DisplayName.Valid {
		disp = &d.DisplayName.String
	}
	var avt *uuid.UUID
	if d.AvatarFileID.Valid {
		avt = &d.AvatarFileID.UUID
	}
	return user.PublicProfile{
		UserID:    d.UserID,
		FirstName: d.FirstName,
		LastName:  d.LastName,
		// Email:        d.Email,
		DisplayName:  disp,
		AvatarFileID: avt,
	}
}
