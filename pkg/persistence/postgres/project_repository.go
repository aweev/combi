package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/combi/pkg/domain/project"
	"github.com/combi/pkg/domain/organization"
	"github.com/combi/pkg/log"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var _ project.Repository = (*ProjectRepository)(nil)

type ProjectRepository struct {
	db     *sqlx.DB
	runner sqlx.ExtContext
	logger log.Logger
	sq     squirrel.StatementBuilderType
}

func NewProjectRepository(db *sqlx.DB, logger log.Logger) *ProjectRepository {
	return &ProjectRepository{
		db:     db,
		runner: db,
		logger: logger.With(log.String("component", "project_repository")),
		sq:     squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

func (r *ProjectRepository) WithTx(ctx context.Context, fn func(txRepo project.Repository) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	txRepo := &ProjectRepository{
		db:     r.db,
		runner: tx,
		logger: r.logger,
		sq:     r.sq,
	}

	if err := fn(txRepo); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

// -----------------------------------------------------------------------------
// Project Repository
// -----------------------------------------------------------------------------

func (r *ProjectRepository) CreateProject(ctx context.Context, p *project.Project) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	p.UpdatedAt = p.CreatedAt

	dbP, err := toDBProject(p)
	if err != nil {
		return err
	}
	if org := organization.OrgFromContext(ctx); org != nil {
		dbP.OrgID = uuid.NullUUID{UUID: org.OrgID, Valid: true}
	}

	query, args, err := r.sq.Insert("projects").
		Columns("id", "org_id", "name", "description", "type", "status", "creator_id", "start_date", "end_date", "created_at", "updated_at").
		Values(dbP.ID, dbP.OrgID, dbP.Name, dbP.Description, dbP.Type, dbP.Status, dbP.CreatorID, dbP.StartDate, dbP.EndDate, dbP.CreatedAt, dbP.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if isProjectUniqueViolation(err) {
			return project.ErrDuplicateProjectName
		}
		return err
	}
	return nil
}

func (r *ProjectRepository) GetProject(ctx context.Context, id uuid.UUID) (*project.Project, error) {
	builder := r.sq.Select("*").From("projects").Where(squirrel.Eq{"id": id})
	builder = WithTenantCol(ctx, builder, "org_id")
	query, args, _ := builder.ToSql()
	var dbP dbProject
	if err := sqlx.GetContext(ctx, r.runner, &dbP, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, project.ErrProjectNotFound
		}
		return nil, err
	}
	return dbP.toDomain(), nil
}

func (r *ProjectRepository) GetProjectByNameAndOrg(ctx context.Context, name string, orgID *uuid.UUID) (*project.Project, error) {
	builder := r.sq.Select("*").From("projects").Where(squirrel.Eq{"name": name})
	if orgID != nil {
		builder = builder.Where(squirrel.Eq{"org_id": *orgID})
	} else {
		builder = builder.Where(squirrel.Eq{"org_id": nil})
	}

	query, args, _ := builder.ToSql()
	var dbP dbProject
	if err := sqlx.GetContext(ctx, r.runner, &dbP, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, project.ErrProjectNotFound
		}
		return nil, err
	}
	return dbP.toDomain(), nil
}

func (r *ProjectRepository) UpdateProject(ctx context.Context, p *project.Project) error {
	p.UpdatedAt = time.Now().UTC()
	dbP, err := toDBProject(p)
	if err != nil {
		return err
	}

	builder := r.sq.Update("projects").
		Set("name", dbP.Name).
		Set("description", dbP.Description).
		Set("status", dbP.Status).
		Set("start_date", dbP.StartDate).
		Set("end_date", dbP.EndDate).
		Set("updated_at", dbP.UpdatedAt).
		Where(squirrel.Eq{"id": dbP.ID})

	builder = WithTenantUpdateCol(ctx, builder, "org_id")
	query, args, err := builder.ToSql()
	if err != nil {
		return err
	}

	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if isProjectUniqueViolation(err) {
			return project.ErrDuplicateProjectName
		}
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return project.ErrProjectNotFound
	}
	return nil
}

func (r *ProjectRepository) ListProjects(ctx context.Context, filter project.ProjectFilter) ([]*project.Project, int, error) {
	builder := r.sq.Select("p.*").From("projects p")
	countBuilder := r.sq.Select("COUNT(DISTINCT p.id)").From("projects p")

	builder = WithTenantCol(ctx, builder, "org_id", "p")
	countBuilder = WithTenantCol(ctx, countBuilder, "org_id", "p")

	if filter.MemberID != nil {
		// Join members to find projects user belongs to
		builder = builder.Join("project_members pm ON pm.project_id = p.id").Where(squirrel.Eq{"pm.user_id": *filter.MemberID})
		countBuilder = countBuilder.Join("project_members pm ON pm.project_id = p.id").Where(squirrel.Eq{"pm.user_id": *filter.MemberID})
	}

	if filter.OrganizationID != nil {
		builder = builder.Where(squirrel.Eq{"p.org_id": *filter.OrganizationID})
		countBuilder = countBuilder.Where(squirrel.Eq{"p.org_id": *filter.OrganizationID})
	}

	if filter.Status != nil {
		builder = builder.Where(squirrel.Eq{"p.status": *filter.Status})
		countBuilder = countBuilder.Where(squirrel.Eq{"p.status": *filter.Status})
	}

	var total int
	cQuery, cArgs, _ := countBuilder.ToSql()
	if err := r.runner.QueryRowxContext(ctx, cQuery, cArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query, args, err := builder.OrderBy("p.updated_at DESC").
		Limit(uint64(filter.Limit)).Offset(uint64(filter.Offset)).ToSql()
	if err != nil {
		return nil, 0, err
	}

	var rows []dbProject
	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, 0, err
	}

	projects := make([]*project.Project, len(rows))
	for i, row := range rows {
		projects[i] = row.toDomain()
	}

	return projects, total, nil
}

// -----------------------------------------------------------------------------
// Member Repository
// -----------------------------------------------------------------------------

func (r *ProjectRepository) CreateMember(ctx context.Context, m *project.ProjectMember) error {
	if m.JoinedAt.IsZero() {
		m.JoinedAt = time.Now().UTC()
	}

	query, args, err := r.sq.Insert("project_members").
		Columns("project_id", "user_id", "role", "joined_at").
		Values(m.ProjectID, m.UserID, m.Role, m.JoinedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		if isProjectUniqueViolation(err) {
			return project.ErrAlreadyAMember
		}
		return err
	}
	return nil
}

func (r *ProjectRepository) GetMember(ctx context.Context, projectID, userID uuid.UUID) (*project.ProjectMember, error) {
	query, args, _ := r.sq.Select("*").From("project_members").
		Where(squirrel.Eq{"project_id": projectID, "user_id": userID}).ToSql()

	var m project.ProjectMember
	if err := sqlx.GetContext(ctx, r.runner, &m, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, project.ErrMemberNotFound
		}
		return nil, err
	}
	return &m, nil
}

func (r *ProjectRepository) UpdateMember(ctx context.Context, m *project.ProjectMember) error {
	query, args, err := r.sq.Update("project_members").
		Set("role", m.Role).
		Where(squirrel.Eq{"project_id": m.ProjectID, "user_id": m.UserID}).
		ToSql()
	if err != nil {
		return err
	}

	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return project.ErrMemberNotFound
	}
	return nil
}

func (r *ProjectRepository) DeleteMember(ctx context.Context, projectID, userID uuid.UUID) error {
	query, args, _ := r.sq.Delete("project_members").
		Where(squirrel.Eq{"project_id": projectID, "user_id": userID}).ToSql()

	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return project.ErrMemberNotFound
	}
	return nil
}

func (r *ProjectRepository) ListMembers(ctx context.Context, projectID uuid.UUID) ([]*project.ProjectMember, error) {
	query, args, _ := r.sq.Select("*").From("project_members").
		Where(squirrel.Eq{"project_id": projectID}).
		OrderBy("joined_at ASC").ToSql()

	var members []*project.ProjectMember
	err := sqlx.SelectContext(ctx, r.runner, &members, query, args...)
	return members, err
}

func (r *ProjectRepository) CountAdmins(ctx context.Context, projectID uuid.UUID) (int, error) {
	query, args, _ := r.sq.Select("COUNT(*)").From("project_members").
		Where(squirrel.Eq{"project_id": projectID, "role": project.RoleAdmin}).ToSql()

	var count int
	err := r.runner.QueryRowxContext(ctx, query, args...).Scan(&count)
	return count, err
}

// -----------------------------------------------------------------------------
// Task Repository
// -----------------------------------------------------------------------------

func (r *ProjectRepository) CreateTask(ctx context.Context, t *project.Task) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}
	t.UpdatedAt = t.CreatedAt

	dbT := toDBTask(t)
	query, args, err := r.sq.Insert("tasks").
		Columns("id", "project_id", "milestone_id", "title", "description", "status", "priority", "creator_id", "assignee_id", "due_date", "created_at", "updated_at").
		Values(dbT.ID, dbT.ProjectID, dbT.MilestoneID, dbT.Title, dbT.Description, dbT.Status, dbT.Priority, dbT.CreatorID, dbT.AssigneeID, dbT.DueDate, dbT.CreatedAt, dbT.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *ProjectRepository) GetTask(ctx context.Context, id uuid.UUID) (*project.Task, error) {
	query, args, _ := r.sq.Select("*").From("tasks").Where(squirrel.Eq{"id": id}).ToSql()
	var dbT dbTask
	if err := sqlx.GetContext(ctx, r.runner, &dbT, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, project.ErrTaskNotFound
		}
		return nil, err
	}
	return dbT.toDomain(), nil
}

func (r *ProjectRepository) UpdateTask(ctx context.Context, t *project.Task) error {
	t.UpdatedAt = time.Now().UTC()
	dbT := toDBTask(t)
	query, args, err := r.sq.Update("tasks").
		Set("title", dbT.Title).
		Set("description", dbT.Description).
		Set("status", dbT.Status).
		Set("priority", dbT.Priority).
		Set("assignee_id", dbT.AssigneeID).
		Set("due_date", dbT.DueDate).
		Set("milestone_id", dbT.MilestoneID).
		Set("updated_at", dbT.UpdatedAt).
		Where(squirrel.Eq{"id": dbT.ID}).
		ToSql()
	if err != nil {
		return err
	}

	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return project.ErrTaskNotFound
	}
	return nil
}

func (r *ProjectRepository) ListTasks(ctx context.Context, projectID uuid.UUID, filter project.TaskFilter) ([]*project.Task, int, error) {
	builder := r.sq.Select("*").From("tasks").Where(squirrel.Eq{"project_id": projectID})
	countBuilder := r.sq.Select("COUNT(*)").From("tasks").Where(squirrel.Eq{"project_id": projectID})

	if filter.Status != nil {
		builder = builder.Where(squirrel.Eq{"status": *filter.Status})
		countBuilder = countBuilder.Where(squirrel.Eq{"status": *filter.Status})
	}
	if filter.AssigneeID != nil {
		builder = builder.Where(squirrel.Eq{"assignee_id": *filter.AssigneeID})
		countBuilder = countBuilder.Where(squirrel.Eq{"assignee_id": *filter.AssigneeID})
	}
	if filter.MilestoneID != nil {
		builder = builder.Where(squirrel.Eq{"milestone_id": *filter.MilestoneID})
		countBuilder = countBuilder.Where(squirrel.Eq{"milestone_id": *filter.MilestoneID})
	}

	var total int
	cQuery, cArgs, _ := countBuilder.ToSql()
	if err := r.runner.QueryRowxContext(ctx, cQuery, cArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query, args, err := builder.OrderBy("created_at DESC").
		Limit(uint64(filter.Limit)).Offset(uint64(filter.Offset)).ToSql()
	if err != nil {
		return nil, 0, err
	}

	var rows []dbTask
	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, 0, err
	}
	tasks := make([]*project.Task, len(rows))
	for i, r := range rows {
		tasks[i] = r.toDomain()
	}
	return tasks, total, nil
}

func (r *ProjectRepository) CountOpenTasksInMilestone(ctx context.Context, milestoneID uuid.UUID) (int, error) {
	query, args, _ := r.sq.Select("COUNT(*)").From("tasks").
		Where(squirrel.Eq{
			"milestone_id": milestoneID,
		}).
		Where(squirrel.NotEq{"status": project.TaskStatusDone}).
		ToSql()

	var count int
	err := r.runner.QueryRowxContext(ctx, query, args...).Scan(&count)
	return count, err
}

// -----------------------------------------------------------------------------
// Milestone Repository
// -----------------------------------------------------------------------------

func (r *ProjectRepository) CreateMilestone(ctx context.Context, m *project.Milestone) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	m.UpdatedAt = m.CreatedAt

	dbM := toDBMilestone(m)
	query, args, err := r.sq.Insert("milestones").
		Columns("id", "project_id", "title", "due_date", "completed_at", "is_critical", "created_at", "updated_at").
		Values(dbM.ID, dbM.ProjectID, dbM.Title, dbM.DueDate, dbM.CompletedAt, dbM.IsCritical, dbM.CreatedAt, dbM.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *ProjectRepository) GetMilestone(ctx context.Context, id uuid.UUID) (*project.Milestone, error) {
	query, args, _ := r.sq.Select("*").From("milestones").Where(squirrel.Eq{"id": id}).ToSql()
	var dbM dbMilestone
	if err := sqlx.GetContext(ctx, r.runner, &dbM, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, project.ErrMilestoneNotFound
		}
		return nil, err
	}
	return dbM.toDomain(), nil
}

func (r *ProjectRepository) UpdateMilestone(ctx context.Context, m *project.Milestone) error {
	m.UpdatedAt = time.Now().UTC()
	dbM := toDBMilestone(m)
	query, args, err := r.sq.Update("milestones").
		Set("title", dbM.Title).
		Set("due_date", dbM.DueDate).
		Set("completed_at", dbM.CompletedAt).
		Set("is_critical", dbM.IsCritical).
		Set("updated_at", dbM.UpdatedAt).
		Where(squirrel.Eq{"id": dbM.ID}).
		ToSql()
	if err != nil {
		return err
	}

	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return project.ErrMilestoneNotFound
	}
	return nil
}

func (r *ProjectRepository) ListMilestones(ctx context.Context, projectID uuid.UUID) ([]*project.Milestone, error) {
	query, args, _ := r.sq.Select("*").From("milestones").
		Where(squirrel.Eq{"project_id": projectID}).
		OrderBy("due_date ASC").ToSql()

	var rows []dbMilestone
	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, err
	}
	ms := make([]*project.Milestone, len(rows))
	for i, r := range rows {
		ms[i] = r.toDomain()
	}
	return ms, nil
}

// -----------------------------------------------------------------------------
// File Repository
// -----------------------------------------------------------------------------

func (r *ProjectRepository) CreateFile(ctx context.Context, f *project.ProjectFile) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	if f.CreatedAt.IsZero() {
		f.CreatedAt = time.Now().UTC()
	}

	dbF := toDBFile(f)
	query, args, err := r.sq.Insert("project_files").
		Columns("id", "project_id", "file_id", "file_name", "file_type", "url", "uploader_id", "description", "created_at").
		Values(dbF.ID, dbF.ProjectID, dbF.FileID, dbF.FileName, dbF.FileType, dbF.URL, dbF.UploaderID, dbF.Description, dbF.CreatedAt).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *ProjectRepository) GetFile(ctx context.Context, id uuid.UUID) (*project.ProjectFile, error) {
	query, args, _ := r.sq.Select("*").From("project_files").Where(squirrel.Eq{"id": id}).ToSql()
	var dbF dbProjectFile
	if err := sqlx.GetContext(ctx, r.runner, &dbF, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, project.ErrFileNotFound
		}
		return nil, err
	}
	return dbF.toDomain(), nil
}

func (r *ProjectRepository) DeleteFile(ctx context.Context, id uuid.UUID) error {
	query, args, _ := r.sq.Delete("project_files").Where(squirrel.Eq{"id": id}).ToSql()
	res, err := r.runner.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return project.ErrFileNotFound
	}
	return nil
}

func (r *ProjectRepository) ListFiles(ctx context.Context, projectID uuid.UUID) ([]*project.ProjectFile, error) {
	query, args, _ := r.sq.Select("*").From("project_files").
		Where(squirrel.Eq{"project_id": projectID}).
		OrderBy("created_at DESC").ToSql()

	var rows []dbProjectFile
	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, err
	}
	files := make([]*project.ProjectFile, len(rows))
	for i, r := range rows {
		files[i] = r.toDomain()
	}
	return files, nil
}

func (r *ProjectRepository) CountFiles(ctx context.Context, projectID uuid.UUID) (int, error) {
	query, args, _ := r.sq.Select("COUNT(*)").From("project_files").Where(squirrel.Eq{"project_id": projectID}).ToSql()
	var count int
	err := r.runner.QueryRowxContext(ctx, query, args...).Scan(&count)
	return count, err
}

// -----------------------------------------------------------------------------
// Activity Repository
// -----------------------------------------------------------------------------

func (r *ProjectRepository) CreateActivity(ctx context.Context, a *project.Activity) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}

	dbA, err := toDBActivity(a)
	if err != nil {
		return err
	}

	query, args, err := r.sq.Insert("project_activities").
		Columns("id", "project_id", "actor_id", "type", "target_type", "target_id", "metadata", "created_at").
		Values(dbA.ID, dbA.ProjectID, dbA.ActorID, dbA.Type, dbA.TargetType, dbA.TargetID, dbA.Metadata, dbA.CreatedAt).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *ProjectRepository) ListActivities(ctx context.Context, projectID uuid.UUID, limit, offset int) ([]*project.Activity, int, error) {
	builder := r.sq.Select("*").From("project_activities").Where(squirrel.Eq{"project_id": projectID})
	countBuilder := r.sq.Select("COUNT(*)").From("project_activities").Where(squirrel.Eq{"project_id": projectID})

	var total int
	cQuery, cArgs, _ := countBuilder.ToSql()
	if err := r.runner.QueryRowxContext(ctx, cQuery, cArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query, args, err := builder.OrderBy("created_at DESC").Limit(uint64(limit)).Offset(uint64(offset)).ToSql()
	if err != nil {
		return nil, 0, err
	}

	var rows []dbActivity
	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, 0, err
	}

	activities := make([]*project.Activity, len(rows))
	for i, r := range rows {
		act, err := r.toDomain()
		if err != nil {
			return nil, 0, err
		}
		activities[i] = act
	}
	return activities, total, nil
}

// -----------------------------------------------------------------------------
// Helpers & DB Models
// -----------------------------------------------------------------------------

func isProjectUniqueViolation(err error) bool {
	if pqErr, ok := err.(*pq.Error); ok {
		return pqErr.Code == "23505"
	}
	return false
}

type dbProject struct {
	ID          uuid.UUID     `db:"id"`
	OrgID       uuid.NullUUID `db:"org_id"`
	Name        string        `db:"name"`
	Description string        `db:"description"`
	Type        string        `db:"type"`
	Status      string        `db:"status"`
	CreatorID   uuid.UUID     `db:"creator_id"`
	StartDate   sql.NullTime  `db:"start_date"`
	EndDate     sql.NullTime  `db:"end_date"`
	CreatedAt   time.Time     `db:"created_at"`
	UpdatedAt   time.Time     `db:"updated_at"`
}

func (d *dbProject) toDomain() *project.Project {
	p := &project.Project{
		ID:          d.ID,
		Name:        d.Name,
		Description: d.Description,
		Type:        project.ProjectType(d.Type),
		Status:      project.ProjectStatus(d.Status),
		CreatorID:   d.CreatorID,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
	if d.OrgID.Valid {
		p.OrganizationID = &d.OrgID.UUID
	}
	if d.StartDate.Valid {
		p.StartDate = &d.StartDate.Time
	}
	if d.EndDate.Valid {
		p.EndDate = &d.EndDate.Time
	}
	return p
}

func toDBProject(p *project.Project) (*dbProject, error) {
	d := &dbProject{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Type:        string(p.Type),
		Status:      string(p.Status),
		CreatorID:   p.CreatorID,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
	if p.OrganizationID != nil {
		d.OrgID = uuid.NullUUID{UUID: *p.OrganizationID, Valid: true}
	}
	if p.StartDate != nil {
		d.StartDate = sql.NullTime{Time: *p.StartDate, Valid: true}
	}
	if p.EndDate != nil {
		d.EndDate = sql.NullTime{Time: *p.EndDate, Valid: true}
	}
	return d, nil
}

type dbTask struct {
	ID          uuid.UUID      `db:"id"`
	ProjectID   uuid.UUID      `db:"project_id"`
	MilestoneID uuid.NullUUID  `db:"milestone_id"`
	Title       string         `db:"title"`
	Description sql.NullString `db:"description"`
	Status      string         `db:"status"`
	Priority    int            `db:"priority"`
	CreatorID   uuid.UUID      `db:"creator_id"`
	AssigneeID  uuid.NullUUID  `db:"assignee_id"`
	DueDate     sql.NullTime   `db:"due_date"`
	CreatedAt   time.Time      `db:"created_at"`
	UpdatedAt   time.Time      `db:"updated_at"`
}

func (d *dbTask) toDomain() *project.Task {
	t := &project.Task{
		ID:        d.ID,
		ProjectID: d.ProjectID,
		Title:     d.Title,
		Status:    project.TaskStatus(d.Status),
		Priority:  d.Priority,
		CreatorID: d.CreatorID,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
	if d.MilestoneID.Valid {
		t.MilestoneID = &d.MilestoneID.UUID
	}
	if d.AssigneeID.Valid {
		t.AssigneeID = &d.AssigneeID.UUID
	}
	if d.Description.Valid {
		t.Description = &d.Description.String
	}
	if d.DueDate.Valid {
		t.DueDate = &d.DueDate.Time
	}
	return t
}

func toDBTask(t *project.Task) *dbTask {
	d := &dbTask{
		ID:        t.ID,
		ProjectID: t.ProjectID,
		Title:     t.Title,
		Status:    string(t.Status),
		Priority:  t.Priority,
		CreatorID: t.CreatorID,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
	if t.MilestoneID != nil {
		d.MilestoneID = uuid.NullUUID{UUID: *t.MilestoneID, Valid: true}
	}
	if t.AssigneeID != nil {
		d.AssigneeID = uuid.NullUUID{UUID: *t.AssigneeID, Valid: true}
	}
	if t.Description != nil {
		d.Description = sql.NullString{String: *t.Description, Valid: true}
	}
	if t.DueDate != nil {
		d.DueDate = sql.NullTime{Time: *t.DueDate, Valid: true}
	}
	return d
}

type dbMilestone struct {
	ID          uuid.UUID    `db:"id"`
	ProjectID   uuid.UUID    `db:"project_id"`
	Title       string       `db:"title"`
	DueDate     time.Time    `db:"due_date"`
	CompletedAt sql.NullTime `db:"completed_at"`
	IsCritical  bool         `db:"is_critical"`
	CreatedAt   time.Time    `db:"created_at"`
	UpdatedAt   time.Time    `db:"updated_at"`
}

func (d *dbMilestone) toDomain() *project.Milestone {
	m := &project.Milestone{
		ID:         d.ID,
		ProjectID:  d.ProjectID,
		Title:      d.Title,
		DueDate:    d.DueDate,
		IsCritical: d.IsCritical,
		CreatedAt:  d.CreatedAt,
		UpdatedAt:  d.UpdatedAt,
	}
	if d.CompletedAt.Valid {
		m.CompletedAt = &d.CompletedAt.Time
	}
	return m
}

func toDBMilestone(m *project.Milestone) *dbMilestone {
	d := &dbMilestone{
		ID:         m.ID,
		ProjectID:  m.ProjectID,
		Title:      m.Title,
		DueDate:    m.DueDate,
		IsCritical: m.IsCritical,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	}
	if m.CompletedAt != nil {
		d.CompletedAt = sql.NullTime{Time: *m.CompletedAt, Valid: true}
	}
	return d
}

type dbProjectFile struct {
	ID          uuid.UUID      `db:"id"`
	ProjectID   uuid.UUID      `db:"project_id"`
	FileID      uuid.UUID      `db:"file_id"`
	FileName    string         `db:"file_name"`
	FileType    string         `db:"file_type"`
	URL         string         `db:"url"`
	UploaderID  uuid.UUID      `db:"uploader_id"`
	Description sql.NullString `db:"description"`
	CreatedAt   time.Time      `db:"created_at"`
}

func (d *dbProjectFile) toDomain() *project.ProjectFile {
	f := &project.ProjectFile{
		ID:         d.ID,
		ProjectID:  d.ProjectID,
		FileID:     d.FileID,
		FileName:   d.FileName,
		FileType:   d.FileType,
		URL:        d.URL,
		UploaderID: d.UploaderID,
		CreatedAt:  d.CreatedAt,
	}
	if d.Description.Valid {
		f.Description = &d.Description.String
	}
	return f
}

func toDBFile(f *project.ProjectFile) *dbProjectFile {
	d := &dbProjectFile{
		ID:         f.ID,
		ProjectID:  f.ProjectID,
		FileID:     f.FileID,
		FileName:   f.FileName,
		FileType:   f.FileType,
		URL:        f.URL,
		UploaderID: f.UploaderID,
		CreatedAt:  f.CreatedAt,
	}
	if f.Description != nil {
		d.Description = sql.NullString{String: *f.Description, Valid: true}
	}
	return d
}

type dbActivity struct {
	ID         uuid.UUID      `db:"id"`
	ProjectID  uuid.UUID      `db:"project_id"`
	ActorID    uuid.UUID      `db:"actor_id"`
	Type       string         `db:"type"`
	TargetType sql.NullString `db:"target_type"`
	TargetID   uuid.NullUUID  `db:"target_id"`
	Metadata   []byte         `db:"metadata"`
	CreatedAt  time.Time      `db:"created_at"`
}

func (d *dbActivity) toDomain() (*project.Activity, error) {
	a := &project.Activity{
		ID:        d.ID,
		ProjectID: d.ProjectID,
		ActorID:   d.ActorID,
		Type:      project.ActivityType(d.Type),
		CreatedAt: d.CreatedAt,
	}
	if d.TargetType.Valid {
		a.TargetType = &d.TargetType.String
	}
	if d.TargetID.Valid {
		a.TargetID = &d.TargetID.UUID
	}
	if len(d.Metadata) > 0 {
		if err := json.Unmarshal(d.Metadata, &a.Metadata); err != nil {
			return nil, err
		}
	}
	return a, nil
}

func toDBActivity(a *project.Activity) (*dbActivity, error) {
	meta, err := json.Marshal(a.Metadata)
	if err != nil {
		return nil, err
	}
	if a.Metadata == nil {
		meta = []byte("{}")
	}

	d := &dbActivity{
		ID:        a.ID,
		ProjectID: a.ProjectID,
		ActorID:   a.ActorID,
		Type:      string(a.Type),
		Metadata:  meta,
		CreatedAt: a.CreatedAt,
	}
	if a.TargetType != nil {
		d.TargetType = sql.NullString{String: *a.TargetType, Valid: true}
	}
	if a.TargetID != nil {
		d.TargetID = uuid.NullUUID{UUID: *a.TargetID, Valid: true}
	}
	return d, nil
}
