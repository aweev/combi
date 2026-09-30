package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/log"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var _ storage.Repository = (*StorageRepository)(nil)

type StorageRepository struct {
	db     *sqlx.DB
	runner sqlx.ExtContext
	logger log.Logger
	sq     squirrel.StatementBuilderType
}

func NewStorageRepository(db *sqlx.DB, logger log.Logger) *StorageRepository {
	return &StorageRepository{
		db:     db,
		runner: db,
		logger: logger.With(log.String("component", "storage_repository")),
		sq:     squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

func (r *StorageRepository) WithTx(ctx context.Context, fn func(txRepo storage.Repository) error) (err error) {
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
	txRepo := &StorageRepository{
		db:     r.db,
		runner: tx,
		logger: r.logger,
		sq:     r.sq,
	}
	return fn(txRepo)
}

// --- File Repository ---

func (r *StorageRepository) CreateFile(ctx context.Context, f *storage.File) error {
	dbModel, err := dbFileFromDomain(f)
	if err != nil {
		return err
	}
	query, args, err := r.sq.Insert("files").
		Columns("id", "owner_id", "owner_type", "bucket_id", "original_name", "path", "mime_type", "size_bytes", "category", "status", "visibility", "hash", "metadata", "tags", "upload_completed_at", "last_accessed_at", "expires_at", "deleted_at", "created_at", "updated_at").
		Values(dbModel.ID, dbModel.OwnerID, dbModel.OwnerType, dbModel.BucketID, dbModel.OriginalName, dbModel.Path, dbModel.MimeType, dbModel.Size, dbModel.Category, dbModel.Status, dbModel.Visibility, dbModel.Hash, dbModel.Metadata, dbModel.Tags, dbModel.UploadCompletedAt, dbModel.LastAccessedAt, dbModel.ExpiresAt, dbModel.DeletedAt, dbModel.CreatedAt, dbModel.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *StorageRepository) GetFile(ctx context.Context, id uuid.UUID) (*storage.File, error) {
	query, args, _ := r.sq.Select("*").From("files").Where(squirrel.Eq{"id": id}).ToSql()
	var dbf dbFile
	if err := sqlx.GetContext(ctx, r.runner, &dbf, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrFileNotFound
		}
		return nil, err
	}
	return dbf.toDomain()
}

func (r *StorageRepository) ListFiles(ctx context.Context, filter storage.FileFilter) ([]*storage.File, int, error) {
	builder := r.sq.Select("*, COUNT(*) OVER() as total_count").From("files")

	if filter.OwnerID != nil {
		builder = builder.Where(squirrel.Eq{"owner_id": *filter.OwnerID})
	}
	if filter.BucketID != nil {
		builder = builder.Where(squirrel.Eq{"bucket_id": *filter.BucketID})
	}
	if filter.Status != nil {
		builder = builder.Where(squirrel.Eq{"status": *filter.Status})
	} else {
		// By default hide deleted
		builder = builder.Where(squirrel.NotEq{"status": storage.StatusDeleted})
	}
	if filter.Category != nil {
		builder = builder.Where(squirrel.Eq{"category": *filter.Category})
	}
	if filter.Search != "" {
		builder = builder.Where(squirrel.ILike{"original_name": "%" + filter.Search + "%"})
	}

	// Pagination
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	offset := (filter.Page - 1) * filter.PageSize

	query, args, err := builder.OrderBy("created_at DESC").Limit(uint64(filter.PageSize)).Offset(uint64(offset)).ToSql()
	if err != nil {
		return nil, 0, err
	}

	var rows []dbFileWithTotal
	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, 0, err
	}

	if len(rows) == 0 {
		return []*storage.File{}, 0, nil
	}

	files := make([]*storage.File, len(rows))
	for i, row := range rows {
		f, err := row.toDomain()
		if err != nil {
			return nil, 0, err
		}
		files[i] = f
	}
	return files, rows[0].Total, nil
}

func (r *StorageRepository) UpdateFile(ctx context.Context, f *storage.File) error {
	f.UpdatedAt = time.Now().UTC()
	dbModel, err := dbFileFromDomain(f)
	if err != nil {
		return err
	}

	query, args, err := r.sq.Update("files").
		Set("original_name", dbModel.OriginalName).
		Set("status", dbModel.Status).
		Set("visibility", dbModel.Visibility).
		Set("metadata", dbModel.Metadata).
		Set("tags", dbModel.Tags).
		Set("upload_completed_at", dbModel.UploadCompletedAt).
		Set("deleted_at", dbModel.DeletedAt).
		Set("updated_at", dbModel.UpdatedAt).
		Set("last_accessed_at", dbModel.LastAccessedAt).
		Set("expires_at", dbModel.ExpiresAt).
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
		return storage.ErrFileNotFound
	}
	return nil
}

func (r *StorageRepository) DeleteFile(ctx context.Context, id uuid.UUID) error {
	query, args, _ := r.sq.Delete("files").Where(squirrel.Eq{"id": id}).ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return storage.ErrFileNotFound
	}
	return nil
}

func (r *StorageRepository) FindSoftDeletedBefore(ctx context.Context, cutoff time.Time) ([]*storage.File, error) {
	query, args, _ := r.sq.Select("*").From("files").
		Where(squirrel.Eq{"status": storage.StatusDeleted}).
		Where(squirrel.Lt{"deleted_at": cutoff}).
		Limit(100). // Batch limit for safety
		ToSql()

	var rows []dbFile
	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, err
	}
	files := make([]*storage.File, len(rows))
	for i, row := range rows {
		f, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		files[i] = f
	}
	return files, nil
}

// --- Upload Repository ---

func (r *StorageRepository) CreateUpload(ctx context.Context, u *storage.Upload) error {
	dbModel := dbUploadFromDomain(u)
	query, args, err := r.sq.Insert("file_uploads").
		Columns("id", "file_id", "provider", "provider_ref", "total_size", "bytes_uploaded", "is_multipart", "expires_at", "created_at").
		Values(dbModel.ID, dbModel.FileID, dbModel.Provider, dbModel.ProviderRef, dbModel.TotalSize, dbModel.BytesUploaded, dbModel.IsMultipart, dbModel.ExpiresAt, dbModel.CreatedAt).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *StorageRepository) GetUpload(ctx context.Context, id uuid.UUID) (*storage.Upload, error) {
	query, args, _ := r.sq.Select("*").From("file_uploads").Where(squirrel.Eq{"id": id}).ToSql()
	var dbu dbUpload
	if err := sqlx.GetContext(ctx, r.runner, &dbu, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrUploadNotFound
		}
		return nil, err
	}
	return dbu.toDomain(), nil
}

func (r *StorageRepository) DeleteUpload(ctx context.Context, id uuid.UUID) error {
	query, args, _ := r.sq.Delete("file_uploads").Where(squirrel.Eq{"id": id}).ToSql()
	_, err := r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *StorageRepository) FindExpiredUploads(ctx context.Context) ([]*storage.Upload, error) {
	query, args, _ := r.sq.Select("*").From("file_uploads").
		Where(squirrel.Lt{"expires_at": time.Now().UTC()}).
		Limit(100).
		ToSql()

	var rows []dbUpload
	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, err
	}
	uploads := make([]*storage.Upload, len(rows))
	for i, r := range rows {
		uploads[i] = r.toDomain()
	}
	return uploads, nil
}

// --- Bucket Repository ---

func (r *StorageRepository) CreateBucket(ctx context.Context, b *storage.StorageBucket) error {
	dbModel := dbBucketFromDomain(b)
	query, args, err := r.sq.Insert("storage_buckets").
		Columns("id", "name", "provider", "region", "endpoint", "is_default", "is_active", "created_at", "updated_at").
		Values(dbModel.ID, dbModel.Name, dbModel.Provider, dbModel.Region, dbModel.Endpoint, dbModel.IsDefault, dbModel.IsActive, dbModel.CreatedAt, dbModel.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return storage.ErrBucketAlreadyExists
		}
		return err
	}
	return nil
}

func (r *StorageRepository) GetBucket(ctx context.Context, id uuid.UUID) (*storage.StorageBucket, error) {
	return r.getBucket(ctx, squirrel.Eq{"id": id})
}

func (r *StorageRepository) GetBucketByName(ctx context.Context, name string) (*storage.StorageBucket, error) {
	return r.getBucket(ctx, squirrel.Eq{"name": name})
}

func (r *StorageRepository) GetDefaultBucket(ctx context.Context) (*storage.StorageBucket, error) {
	return r.getBucket(ctx, squirrel.Eq{"is_default": true})
}

func (r *StorageRepository) getBucket(ctx context.Context, pred interface{}) (*storage.StorageBucket, error) {
	query, args, _ := r.sq.Select("*").From("storage_buckets").Where(pred).ToSql()
	var dbb dbBucket
	if err := sqlx.GetContext(ctx, r.runner, &dbb, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrBucketNotFound
		}
		return nil, err
	}
	return dbb.toDomain(), nil
}

func (r *StorageRepository) ListBuckets(ctx context.Context) ([]*storage.StorageBucket, error) {
	query, args, _ := r.sq.Select("*").From("storage_buckets").OrderBy("name ASC").ToSql()
	var rows []dbBucket
	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, err
	}
	buckets := make([]*storage.StorageBucket, len(rows))
	for i, r := range rows {
		buckets[i] = r.toDomain()
	}
	return buckets, nil
}

func (r *StorageRepository) UpdateBucket(ctx context.Context, b *storage.StorageBucket) error {
	b.UpdatedAt = time.Now().UTC()
	dbModel := dbBucketFromDomain(b)
	query, args, err := r.sq.Update("storage_buckets").
		Set("is_default", dbModel.IsDefault).
		Set("is_active", dbModel.IsActive).
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
		return storage.ErrBucketNotFound
	}
	return nil
}

func (r *StorageRepository) DeleteBucket(ctx context.Context, id uuid.UUID) error {
	query, args, _ := r.sq.Delete("storage_buckets").Where(squirrel.Eq{"id": id}).ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return storage.ErrBucketNotFound
	}
	return nil
}

// --- Quota Repository ---

func (r *StorageRepository) GetOrCreateQuota(ctx context.Context, ownerID uuid.UUID, ownerType string, defaultLimit int64) (*storage.StorageQuota, error) {
	// Try Get first
	query, args, _ := r.sq.Select("*").From("storage_quotas").
		Where(squirrel.Eq{"owner_id": ownerID, "owner_type": ownerType}).ToSql()
	var dbq dbQuota
	err := sqlx.GetContext(ctx, r.runner, &dbq, query, args...)
	if err == nil {
		return dbq.toDomain(), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	// Create if not exists
	q := &storage.StorageQuota{
		ID:        uuid.New(),
		OwnerID:   ownerID,
		OwnerType: ownerType,
		Limit:     defaultLimit,
		Used:      0,
	}

	insQuery, insArgs, _ := r.sq.Insert("storage_quotas").
		Columns("id", "owner_id", "owner_type", "limit_bytes", "used_bytes", "updated_at").
		Values(q.ID, q.OwnerID, q.OwnerType, q.Limit, q.Used, time.Now().UTC()).
		Suffix("ON CONFLICT (owner_id, owner_type) DO NOTHING"). // Handle race
		ToSql()

	_, err = r.runner.ExecContext(ctx, insQuery, insArgs...)
	if err != nil {
		return nil, err
	}

	// Fetch again to ensure we return the committed record
	return r.GetOrCreateQuota(ctx, ownerID, ownerType, defaultLimit)
}

func (r *StorageRepository) UpdateQuota(ctx context.Context, q *storage.StorageQuota) error {
	q.UpdatedAt = time.Now().UTC()
	query, args, _ := r.sq.Update("storage_quotas").
		Set("limit_bytes", q.Limit).
		Set("updated_at", q.UpdatedAt).
		Where(squirrel.Eq{"id": q.ID}).
		ToSql()
	_, err := r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *StorageRepository) IncrementUsage(ctx context.Context, ownerID uuid.UUID, ownerType string, size int64) error {
	query, args, _ := r.sq.Update("storage_quotas").
		Set("used_bytes", squirrel.Expr("used_bytes + ?", size)).
		Set("updated_at", time.Now().UTC()).
		Where(squirrel.Eq{"owner_id": ownerID, "owner_type": ownerType}).
		ToSql()
	_, err := r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *StorageRepository) DecrementUsage(ctx context.Context, ownerID uuid.UUID, ownerType string, size int64) error {
	query, args, _ := r.sq.Update("storage_quotas").
		Set("used_bytes", squirrel.Expr("GREATEST(0, used_bytes - ?)", size)).
		Set("updated_at", time.Now().UTC()).
		Where(squirrel.Eq{"owner_id": ownerID, "owner_type": ownerType}).
		ToSql()
	_, err := r.runner.ExecContext(ctx, query, args...)
	return err
}

// --- Models ---

type dbFile struct {
	ID                uuid.UUID      `db:"id"`
	OwnerID           uuid.UUID      `db:"owner_id"`
	OwnerType         string         `db:"owner_type"`
	BucketID          uuid.UUID      `db:"bucket_id"`
	OriginalName      string         `db:"original_name"`
	Path              string         `db:"path"`
	MimeType          string         `db:"mime_type"`
	Size              int64          `db:"size_bytes"`
	Category          string         `db:"category"`
	Status            string         `db:"status"`
	Visibility        string         `db:"visibility"`
	Hash              sql.NullString `db:"hash"`
	Metadata          []byte         `db:"metadata"`
	Tags              pq.StringArray `db:"tags"`
	UploadCompletedAt sql.NullTime   `db:"upload_completed_at"`
	LastAccessedAt    sql.NullTime   `db:"last_accessed_at"`
	ExpiresAt         sql.NullTime   `db:"expires_at"`
	DeletedAt         sql.NullTime   `db:"deleted_at"`
	CreatedAt         time.Time      `db:"created_at"`
	UpdatedAt         time.Time      `db:"updated_at"`
}

type dbFileWithTotal struct {
	dbFile
	Total int `db:"total_count"`
}

func dbFileFromDomain(f *storage.File) (*dbFile, error) {
	meta := f.CustomMetadata
	if meta == nil {
		meta = make(map[string]interface{})
	}
	if f.Width != nil {
		meta["width"] = *f.Width
	}
	if f.Height != nil {
		meta["height"] = *f.Height
	}
	if f.Duration != nil {
		meta["duration"] = *f.Duration
	}
	if f.Description != nil {
		meta["description"] = *f.Description
	}

	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}

	return &dbFile{
		ID:                f.ID,
		OwnerID:           f.OwnerID,
		OwnerType:         f.OwnerType,
		BucketID:          f.BucketID,
		OriginalName:      f.OriginalName,
		Path:              f.Path,
		MimeType:          f.MimeType,
		Size:              f.Size,
		Category:          string(f.Category),
		Status:            string(f.Status),
		Visibility:        string(f.Visibility),
		Hash:              toNullString(f.Hash),
		Metadata:          metaBytes,
		Tags:              pq.StringArray(f.Tags),
		UploadCompletedAt: toNullTime(f.UploadCompletedAt),
		LastAccessedAt:    toNullTime(f.LastAccessedAt), // Map
		ExpiresAt:         toNullTime(f.ExpiresAt),      // Map
		DeletedAt:         toNullTime(f.DeletedAt),
		CreatedAt:         f.CreatedAt,
		UpdatedAt:         f.UpdatedAt,
	}, nil
}

func (d *dbFile) toDomain() (*storage.File, error) {
	var meta map[string]interface{}
	if err := json.Unmarshal(d.Metadata, &meta); err != nil {
		return nil, err
	}

	var width, height *int
	var dur *float64
	var desc *string

	if w, ok := meta["width"].(float64); ok {
		v := int(w)
		width = &v
	}
	if h, ok := meta["height"].(float64); ok {
		v := int(h)
		height = &v
	}
	if d, ok := meta["duration"].(float64); ok {
		dur = &d
	}
	if descStr, ok := meta["description"].(string); ok {
		desc = &descStr
	}

	var hash *string
	if d.Hash.Valid {
		hash = &d.Hash.String
	}

	var upAt, accAt, expAt, delAt *time.Time
	if d.UploadCompletedAt.Valid {
		upAt = &d.UploadCompletedAt.Time
	}
	if d.LastAccessedAt.Valid {
		accAt = &d.LastAccessedAt.Time
	} // Map
	if d.ExpiresAt.Valid {
		expAt = &d.ExpiresAt.Time
	} // Map
	if d.DeletedAt.Valid {
		delAt = &d.DeletedAt.Time
	}

	return &storage.File{
		ID:                d.ID,
		OwnerID:           d.OwnerID,
		OwnerType:         d.OwnerType,
		BucketID:          d.BucketID,
		OriginalName:      d.OriginalName,
		Path:              d.Path,
		MimeType:          d.MimeType,
		Size:              d.Size,
		Category:          storage.FileCategory(d.Category),
		Status:            storage.FileStatus(d.Status),
		Visibility:        storage.FileVisibility(d.Visibility),
		Hash:              hash,
		Width:             width,
		Height:            height,
		Duration:          dur,
		Description:       desc,
		CustomMetadata:    meta,
		Tags:              []string(d.Tags),
		UploadCompletedAt: upAt,
		LastAccessedAt:    accAt,
		ExpiresAt:         expAt,
		DeletedAt:         delAt,
		CreatedAt:         d.CreatedAt,
		UpdatedAt:         d.UpdatedAt,
	}, nil
}

type dbBucket struct {
	ID        uuid.UUID      `db:"id"`
	Name      string         `db:"name"`
	Provider  string         `db:"provider"`
	Region    string         `db:"region"`
	Endpoint  sql.NullString `db:"endpoint"`
	IsDefault bool           `db:"is_default"`
	IsActive  bool           `db:"is_active"`
	CreatedAt time.Time      `db:"created_at"`
	UpdatedAt time.Time      `db:"updated_at"`
}

func dbBucketFromDomain(b *storage.StorageBucket) *dbBucket {
	return &dbBucket{
		ID:        b.ID,
		Name:      b.Name,
		Provider:  string(b.Provider),
		Region:    b.Region,
		Endpoint:  toNullString(b.Endpoint),
		IsDefault: b.IsDefault,
		IsActive:  b.IsActive,
		CreatedAt: b.CreatedAt,
		UpdatedAt: b.UpdatedAt,
	}
}

func (d *dbBucket) toDomain() *storage.StorageBucket {
	var ep *string
	if d.Endpoint.Valid {
		ep = &d.Endpoint.String
	}
	return &storage.StorageBucket{
		ID:        d.ID,
		Name:      d.Name,
		Provider:  storage.StorageProvider(d.Provider),
		Region:    d.Region,
		Endpoint:  ep,
		IsDefault: d.IsDefault,
		IsActive:  d.IsActive,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}

type dbQuota struct {
	ID        uuid.UUID `db:"id"`
	OwnerID   uuid.UUID `db:"owner_id"`
	OwnerType string    `db:"owner_type"`
	Limit     int64     `db:"limit_bytes"`
	Used      int64     `db:"used_bytes"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (d *dbQuota) toDomain() *storage.StorageQuota {
	return &storage.StorageQuota{
		ID:        d.ID,
		OwnerID:   d.OwnerID,
		OwnerType: d.OwnerType,
		Limit:     d.Limit,
		Used:      d.Used,
		UpdatedAt: d.UpdatedAt,
	}
}

type dbUpload struct {
	ID            uuid.UUID      `db:"id"`
	FileID        uuid.UUID      `db:"file_id"`
	Provider      string         `db:"provider"`
	ProviderRef   sql.NullString `db:"provider_ref"`
	TotalSize     int64          `db:"total_size"`
	BytesUploaded int64          `db:"bytes_uploaded"`
	IsMultipart   bool           `db:"is_multipart"`
	ExpiresAt     time.Time      `db:"expires_at"`
	CreatedAt     time.Time      `db:"created_at"`
}

func dbUploadFromDomain(u *storage.Upload) *dbUpload {
	var ref sql.NullString
	if u.ProviderRef != "" {
		ref.String = u.ProviderRef
		ref.Valid = true
	}
	return &dbUpload{
		ID:            u.ID,
		FileID:        u.FileID,
		Provider:      string(u.Provider),
		ProviderRef:   ref,
		TotalSize:     u.TotalSize,
		BytesUploaded: u.BytesUploaded,
		IsMultipart:   u.IsMultipart,
		ExpiresAt:     u.ExpiresAt,
		CreatedAt:     u.CreatedAt,
	}
}

func (d *dbUpload) toDomain() *storage.Upload {
	return &storage.Upload{
		ID:            d.ID,
		FileID:        d.FileID,
		Provider:      storage.StorageProvider(d.Provider),
		ProviderRef:   d.ProviderRef.String,
		TotalSize:     d.TotalSize,
		BytesUploaded: d.BytesUploaded,
		IsMultipart:   d.IsMultipart,
		ExpiresAt:     d.ExpiresAt,
		CreatedAt:     d.CreatedAt,
	}
}
