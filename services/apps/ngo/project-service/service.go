package project

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/project"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/reporting"
	"github.com/combi/pkg/domain/sync_domain"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

var _ project.Service = (*Service)(nil)
var _ reporting.DataFetcher = (*Service)(nil)
var _ sync_domain.SyncableService = (*Service)(nil)

type Service struct {
	repo          project.Repository
	auditRecorder audit.EventRecorder
	eventBus      events.Bus
	authz         rbac.AuthorizationService
	logger        log.Logger
}

func NewService(
	repo project.Repository,
	audit audit.EventRecorder,
	eventBus events.Bus,
	authz rbac.AuthorizationService,
	logger log.Logger,
) *Service {
	return &Service{
		repo:          repo,
		auditRecorder: audit,
		eventBus:      eventBus,
		authz:         authz,
		logger:        logger.With(log.String("service", "project")),
	}
}

func (s *Service) FetchData(ctx context.Context, params map[string]any) (any, error) {
	reportType, _ := params["type"].(string)

	switch reportType {
	case "PROJECT_STATUS_SUMMARY":
		// 1. Extract params
		orgIDStr, _ := params["organization_id"].(string)
		if orgIDStr == "" {
			return nil, fmt.Errorf("organization_id required")
		}
		orgID := uuid.MustParse(orgIDStr)

		// 2. Query (Using Repository)
		// We might need a new repo method for aggregation, or reuse ListProjects
		projects, _, err := s.repo.ListProjects(ctx, project.ProjectFilter{
			OrganizationID: &orgID,
			Limit:          1000,
		})
		if err != nil {
			return nil, err
		}

		// 3. Transform to DTO
		type ProjectSummary struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			Tasks  int    `json:"task_count"` // Ideally fetched via aggregation
		}

		summaries := make([]ProjectSummary, len(projects))
		for i, p := range projects {
			// Note: N+1 query here if we fetch tasks counts.
			// Better to add aggregation method to repo.
			summaries[i] = ProjectSummary{
				Name:   p.Name,
				Status: string(p.Status),
			}
		}
		return summaries, nil

	default:
		return nil, fmt.Errorf("unknown report type: %s", reportType)
	}
}

// =============================================================================
// PROJECT LIFECYCLE
// =============================================================================

func (s *Service) CreateProject(ctx context.Context, params project.CreateProjectParams) (*project.Project, error) {
	// 1. Auth Check (Can create in org?)
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, auth.ErrUnauthorized
	}

	// 2. Prepare Project
	proj := &project.Project{
		ID:             uuid.New(),
		OrganizationID: params.OrganizationID,
		Name:           params.Name,
		Description:    params.Description,
		Type:           params.Type,
		Status:         project.StatusPlanning,
		CreatorID:      actor.UserID,
		StartDate:      params.StartDate,
		EndDate:        params.EndDate,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	// 3. Transaction: Create Project + Add Creator as Admin
	err := s.repo.WithTx(ctx, func(txRepo project.Repository) error {
		if err := txRepo.CreateProject(ctx, proj); err != nil {
			return err
		}

		member := &project.ProjectMember{
			ProjectID: proj.ID,
			UserID:    actor.UserID,
			Role:      project.RoleAdmin,
			JoinedAt:  time.Now().UTC(),
		}
		if err := txRepo.CreateMember(ctx, member); err != nil {
			return err
		}

		// Initial Activity
		return s.logActivity(ctx, txRepo, proj.ID, actor.UserID, project.ActivityProjectCreated, nil, nil, nil)
	})

	if err != nil {
		return nil, err
	}

	s.audit(ctx, actor.UserID, core.EventProjectCreated, proj.ID, proj.Name)

	if err := s.publishEvent(ctx, core.EventProjectCreated, actor, proj.ID, "project", proj); err != nil {
		s.logger.Error("failed to publish project created event", err, log.String("project_id", proj.ID.String()))
	}

	return proj, nil
}

func (s *Service) GetProject(ctx context.Context, projectID uuid.UUID) (*project.Project, error) {
	if err := s.checkAccess(ctx, projectID, project.PermProjectRead); err != nil {
		return nil, err
	}
	return s.repo.GetProject(ctx, projectID)
}

func (s *Service) GetProjectDetails(ctx context.Context, projectID uuid.UUID) (*project.ProjectDetails, error) {
	if err := s.checkAccess(ctx, projectID, project.PermProjectRead); err != nil {
		return nil, err
	}

	proj, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}

	members, _ := s.repo.ListMembers(ctx, projectID)
	tasks, _, _ := s.repo.ListTasks(ctx, projectID, project.TaskFilter{Limit: 50})
	milestones, _ := s.repo.ListMilestones(ctx, projectID)
	files, _ := s.repo.ListFiles(ctx, projectID)

	details := &project.ProjectDetails{
		Project:    *proj,
		Members:    members,
		Tasks:      tasks,
		Milestones: milestones,
		Files:      files,
	}

	return details, nil
}

func (s *Service) ListProjects(ctx context.Context, filter project.ProjectFilter) ([]*project.Project, int, error) {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return nil, 0, auth.ErrUnauthorized
	}

	// Security: Filter to projects the user is a member of.
	// Future enhancement: Allow Org Admins to see all projects in Org.
	filter.MemberID = &actor.UserID

	return s.repo.ListProjects(ctx, filter)
}

func (s *Service) UpdateProject(ctx context.Context, projectID uuid.UUID, params project.UpdateProjectParams) (*project.Project, error) {
	if err := s.checkAccess(ctx, projectID, project.PermProjectUpdate); err != nil {
		return nil, err
	}

	var updatedProject *project.Project

	err := s.repo.WithTx(ctx, func(txRepo project.Repository) error {
		proj, err := txRepo.GetProject(ctx, projectID)
		if err != nil {
			return err
		}

		// Apply Updates
		if params.Name != nil {
			proj.Name = *params.Name
		}
		if params.Description != nil {
			proj.Description = *params.Description
		}
		if params.StartDate != nil {
			proj.StartDate = params.StartDate
		}
		if params.EndDate != nil {
			proj.EndDate = params.EndDate
		}

		statusChanged := false
		if params.Status != nil && *params.Status != proj.Status {
			proj.Status = *params.Status
			statusChanged = true
		}

		if err := txRepo.UpdateProject(ctx, proj); err != nil {
			return err
		}

		if statusChanged {
			actor := auth.ActorFromContext(ctx)
			if err := s.logActivity(ctx, txRepo, projectID, actor.UserID, project.ActivityProjectStatusChanged, nil, nil, map[string]any{"new_status": proj.Status}); err != nil {
				s.logger.Error("failed to log project status change activity", err, log.String("project_id", projectID.String()))
			}
		}

		updatedProject = proj
		return nil
	})

	if err != nil {
		return nil, err
	}

	return updatedProject, nil
}

func (s *Service) ArchiveProject(ctx context.Context, projectID uuid.UUID) (*project.Project, error) {
	if err := s.checkAccess(ctx, projectID, project.PermProjectDelete); err != nil {
		return nil, err
	}

	var archived *project.Project
	err := s.repo.WithTx(ctx, func(txRepo project.Repository) error {
		proj, err := txRepo.GetProject(ctx, projectID)
		if err != nil {
			return err
		}

		proj.Status = project.StatusArchived
		if err := txRepo.UpdateProject(ctx, proj); err != nil {
			return err
		}
		archived = proj
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.audit(ctx, auth.ActorFromContext(ctx).UserID, core.EventProjectArchived, projectID, archived.Name)
	return archived, nil
}

// =============================================================================
// MEMBER MANAGEMENT
// =============================================================================

func (s *Service) AddMember(ctx context.Context, projectID, userID uuid.UUID, role project.ProjectMemberRole) (*project.ProjectMember, error) {
	if err := s.checkAccess(ctx, projectID, project.PermProjectManageMembers); err != nil {
		return nil, err
	}

	member := &project.ProjectMember{
		ProjectID: projectID,
		UserID:    userID,
		Role:      role,
		JoinedAt:  time.Now().UTC(),
	}

	err := s.repo.WithTx(ctx, func(txRepo project.Repository) error {
		if err := txRepo.CreateMember(ctx, member); err != nil {
			return err
		}

		actor := auth.ActorFromContext(ctx)
		targetType := "USER"
		return s.logActivity(ctx, txRepo, projectID, actor.UserID, project.ActivityMemberAdded, &targetType, &userID, map[string]any{"role": role})
	})

	if err != nil {
		return nil, err
	}

	if err := s.publishEvent(ctx, core.EventProjectMemberAdded, auth.ActorFromContext(ctx), projectID, "project", map[string]string{
		"user_id": userID.String(),
		"role":    string(role),
	}); err != nil {
		s.logger.Error("failed to publish member added event", err, log.String("project_id", projectID.String()))
	}

	return member, nil
}

func (s *Service) RemoveMember(ctx context.Context, projectID, userID uuid.UUID) error {
	if err := s.checkAccess(ctx, projectID, project.PermProjectManageMembers); err != nil {
		return err
	}

	return s.repo.WithTx(ctx, func(txRepo project.Repository) error {
		// Rule: Cannot remove the last Admin
		member, err := txRepo.GetMember(ctx, projectID, userID)
		if err != nil {
			return err
		}
		if member.Role == project.RoleAdmin {
			count, _ := txRepo.CountAdmins(ctx, projectID)
			if count <= 1 {
				return project.ErrCannotRemoveLastAdmin
			}
		}

		if err := txRepo.DeleteMember(ctx, projectID, userID); err != nil {
			return err
		}

		actor := auth.ActorFromContext(ctx)
		targetType := "USER"
		return s.logActivity(ctx, txRepo, projectID, actor.UserID, project.ActivityMemberRemoved, &targetType, &userID, nil)
	})
}

func (s *Service) UpdateMemberRole(ctx context.Context, projectID, userID uuid.UUID, role project.ProjectMemberRole) (*project.ProjectMember, error) {
	if err := s.checkAccess(ctx, projectID, project.PermProjectManageMembers); err != nil {
		return nil, err
	}

	var updatedMember *project.ProjectMember
	err := s.repo.WithTx(ctx, func(txRepo project.Repository) error {
		member, err := txRepo.GetMember(ctx, projectID, userID)
		if err != nil {
			return err
		}

		// Rule: Cannot downgrade the last Admin
		if member.Role == project.RoleAdmin && role != project.RoleAdmin {
			count, _ := txRepo.CountAdmins(ctx, projectID)
			if count <= 1 {
				return project.ErrCannotRemoveLastAdmin
			}
		}

		member.Role = role
		if err := txRepo.UpdateMember(ctx, member); err != nil {
			return err
		}
		updatedMember = member
		return nil
	})

	if err != nil {
		return nil, err
	}

	return updatedMember, nil
}

func (s *Service) ListMembers(ctx context.Context, projectID uuid.UUID) ([]*project.ProjectMember, error) {
	if err := s.checkAccess(ctx, projectID, project.PermProjectRead); err != nil {
		return nil, err
	}
	return s.repo.ListMembers(ctx, projectID)
}

func (s *Service) IsMember(ctx context.Context, projectID, userID uuid.UUID) (bool, error) {
	_, err := s.repo.GetMember(ctx, projectID, userID)
	if err == nil {
		return true, nil
	}
	if err == project.ErrMemberNotFound {
		return false, nil
	}
	return false, err
}

// =============================================================================
// TASK MANAGEMENT
// =============================================================================

func (s *Service) CreateTask(ctx context.Context, params project.CreateTaskParams) (*project.Task, error) {
	if err := s.checkAccess(ctx, params.ProjectID, project.PermProjectTaskCreate); err != nil {
		return nil, err
	}

	task := &project.Task{
		ID:          uuid.New(),
		ProjectID:   params.ProjectID,
		MilestoneID: params.MilestoneID,
		Title:       params.Title,
		Description: params.Description,
		Status:      project.TaskStatusTodo,
		Priority:    params.Priority,
		CreatorID:   params.CreatorID,
		AssigneeID:  params.AssigneeID,
		DueDate:     params.DueDate,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	err := s.repo.WithTx(ctx, func(txRepo project.Repository) error {
		if err := txRepo.CreateTask(ctx, task); err != nil {
			return err
		}

		targetType := "TASK"
		return s.logActivity(ctx, txRepo, params.ProjectID, params.CreatorID, project.ActivityTaskCreated, &targetType, &task.ID, map[string]any{"title": task.Title})
	})

	if err != nil {
		return nil, err
	}

	return task, nil
}

func (s *Service) UpdateTask(ctx context.Context, taskID uuid.UUID, params project.UpdateTaskParams) (*project.Task, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}

	if err := s.checkAccess(ctx, task.ProjectID, project.PermProjectTaskUpdate); err != nil {
		return nil, err
	}

	if params.Title != nil {
		task.Title = *params.Title
	}
	if params.Description != nil {
		task.Description = params.Description
	}
	if params.Status != nil {
		task.Status = *params.Status
	}
	if params.Priority != nil {
		task.Priority = *params.Priority
	}
	if params.AssigneeID != nil {
		task.AssigneeID = params.AssigneeID
	}
	if params.DueDate != nil {
		task.DueDate = params.DueDate
	}

	if err := s.repo.UpdateTask(ctx, task); err != nil {
		return nil, err
	}

	return task, nil
}

func (s *Service) GetTask(ctx context.Context, taskID uuid.UUID) (*project.Task, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if err := s.checkAccess(ctx, task.ProjectID, project.PermProjectRead); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *Service) MoveTaskStatus(ctx context.Context, taskID uuid.UUID, newStatus project.TaskStatus) (*project.Task, error) {
	var updatedTask *project.Task

	err := s.repo.WithTx(ctx, func(txRepo project.Repository) error {
		task, err := txRepo.GetTask(ctx, taskID)
		if err != nil {
			return err
		}
		// Using the closure to capture service ref to call checkAccess is tricky inside WithTx if checkAccess uses s.repo.
		// Since checkAccess uses s.repo (which might be the DB pool), it's fine for READS.
		// However, for consistency, we assume READS are safe outside the Tx for permission checks.
		if err := s.checkAccess(ctx, task.ProjectID, project.PermProjectTaskUpdate); err != nil {
			return err
		}

		task.Status = newStatus
		if err := txRepo.UpdateTask(ctx, task); err != nil {
			return err
		}

		actor := auth.ActorFromContext(ctx)
		targetType := "TASK"
		if err := s.logActivity(ctx, txRepo, task.ProjectID, actor.UserID, project.ActivityTaskStatusChanged, &targetType, &task.ID, map[string]any{"status": newStatus}); err != nil {
			return err
		}

		updatedTask = task
		return nil
	})

	if err != nil {
		return nil, err
	}

	return updatedTask, nil
}

func (s *Service) ListTasks(ctx context.Context, projectID uuid.UUID, filter project.TaskFilter) ([]*project.Task, int, error) {
	if err := s.checkAccess(ctx, projectID, project.PermProjectRead); err != nil {
		return nil, 0, err
	}
	return s.repo.ListTasks(ctx, projectID, filter)
}

// =============================================================================
// MILESTONE MANAGEMENT
// =============================================================================

func (s *Service) CreateMilestone(ctx context.Context, params project.CreateMilestoneParams) (*project.Milestone, error) {
	if err := s.checkAccess(ctx, params.ProjectID, project.PermProjectUpdate); err != nil {
		return nil, err
	}

	m := &project.Milestone{
		ID:         uuid.New(),
		ProjectID:  params.ProjectID,
		Title:      params.Title,
		DueDate:    params.DueDate,
		IsCritical: params.IsCritical,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}

	if err := s.repo.CreateMilestone(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *Service) UpdateMilestone(ctx context.Context, id uuid.UUID, params project.UpdateMilestoneParams) (*project.Milestone, error) {
	m, err := s.repo.GetMilestone(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.checkAccess(ctx, m.ProjectID, project.PermProjectUpdate); err != nil {
		return nil, err
	}

	if params.Title != nil {
		m.Title = *params.Title
	}
	if params.DueDate != nil {
		m.DueDate = *params.DueDate
	}
	if params.IsCritical != nil {
		m.IsCritical = *params.IsCritical
	}

	if err := s.repo.UpdateMilestone(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *Service) CompleteMilestone(ctx context.Context, id uuid.UUID) error {
	m, err := s.repo.GetMilestone(ctx, id)
	if err != nil {
		return err
	}
	if err := s.checkAccess(ctx, m.ProjectID, project.PermProjectUpdate); err != nil {
		return err
	}

	openTasks, err := s.repo.CountOpenTasksInMilestone(ctx, id)
	if err != nil {
		return err
	}
	if openTasks > 0 {
		return project.ErrMilestoneIncomplete
	}

	now := time.Now().UTC()
	m.CompletedAt = &now
	return s.repo.UpdateMilestone(ctx, m)
}

func (s *Service) ListMilestones(ctx context.Context, projectID uuid.UUID) ([]*project.Milestone, error) {
	if err := s.checkAccess(ctx, projectID, project.PermProjectRead); err != nil {
		return nil, err
	}
	return s.repo.ListMilestones(ctx, projectID)
}

// =============================================================================
// FILE MANAGEMENT
// =============================================================================

func (s *Service) AttachFile(ctx context.Context, params project.AttachFileParams) (*project.ProjectFile, error) {
	if err := s.checkAccess(ctx, params.ProjectID, project.PermProjectFileUpload); err != nil {
		return nil, err
	}

	file := &project.ProjectFile{
		ID:          uuid.New(),
		ProjectID:   params.ProjectID,
		FileID:      params.FileID,
		FileName:    params.FileName,
		FileType:    params.FileType,
		URL:         params.URL,
		UploaderID:  params.UploaderID,
		Description: params.Description,
		CreatedAt:   time.Now().UTC(),
	}

	err := s.repo.WithTx(ctx, func(txRepo project.Repository) error {
		if err := txRepo.CreateFile(ctx, file); err != nil {
			return err
		}

		targetType := "FILE"
		return s.logActivity(ctx, txRepo, params.ProjectID, params.UploaderID, project.ActivityFileUploaded, &targetType, &file.ID, map[string]any{"name": file.FileName})
	})

	if err != nil {
		return nil, err
	}

	return file, nil
}

func (s *Service) ListFiles(ctx context.Context, projectID uuid.UUID) ([]*project.ProjectFile, error) {
	if err := s.checkAccess(ctx, projectID, project.PermProjectRead); err != nil {
		return nil, err
	}
	return s.repo.ListFiles(ctx, projectID)
}

func (s *Service) RemoveFile(ctx context.Context, fileID uuid.UUID) error {
	file, err := s.repo.GetFile(ctx, fileID)
	if err != nil {
		return err
	}
	if err := s.checkAccess(ctx, file.ProjectID, project.PermProjectFileDelete); err != nil {
		return err
	}

	return s.repo.DeleteFile(ctx, fileID)
}

// =============================================================================
// ACTIVITY
// =============================================================================

func (s *Service) ListActivity(ctx context.Context, projectID uuid.UUID, limit, offset int) ([]*project.Activity, int, error) {
	if err := s.checkAccess(ctx, projectID, project.PermProjectRead); err != nil {
		return nil, 0, err
	}
	return s.repo.ListActivities(ctx, projectID, limit, offset)
}

// ApplySyncChange handles offline task creation and updates.
func (s *Service) ApplySyncChange(ctx context.Context, actorID uuid.UUID, op sync_domain.SyncOperation) (uuid.UUID, int, error) {
	if op.EntityType != "task" {
		return uuid.Nil, 0, fmt.Errorf("project service only supports syncing 'task' entities, got: %s", op.EntityType)
	}

	// Inject Actor into Context for Service methods that rely on ActorFromContext
	actor := &auth.Actor{UserID: actorID}
	userCtx := auth.NewContextWithActor(ctx, actor)

	switch op.Action {
	case sync_domain.ActionCreate:
		var req struct {
			ProjectID   uuid.UUID  `json:"project_id"`
			Title       string     `json:"title"`
			Description string     `json:"description"`
			AssigneeID  *uuid.UUID `json:"assignee_id"`
			DueDate     *time.Time `json:"due_date"`
		}
		if err := json.Unmarshal(op.Payload, &req); err != nil {
			return uuid.Nil, 0, fmt.Errorf("invalid payload: %w", err)
		}

		// Use existing service method which handles permission checks & audit
		task, err := s.CreateTask(userCtx, project.CreateTaskParams{
			ProjectID:   req.ProjectID,
			Title:       req.Title,
			Description: &req.Description,
			CreatorID:   actorID,
			AssigneeID:  req.AssigneeID,
			DueDate:     req.DueDate,
		})
		if err != nil {
			return uuid.Nil, 0, err
		}
		return task.ID, 1, nil // Version 1

	case sync_domain.ActionUpdate:
		if op.EntityID == nil {
			return uuid.Nil, 0, fmt.Errorf("entity_id required for update")
		}
		taskID := *op.EntityID

		var req struct {
			Status      *project.TaskStatus `json:"status"`
			Title       *string             `json:"title"`
			Description *string             `json:"description"`
			AssigneeID  *uuid.UUID          `json:"assignee_id"`
		}
		if err := json.Unmarshal(op.Payload, &req); err != nil {
			return uuid.Nil, 0, fmt.Errorf("invalid payload: %w", err)
		}

		// Handle Status separately if needed, or via generic update
		// The service has `MoveTaskStatus` and `UpdateTask`.
		// Let's use `UpdateTask` for general fields and handle status if present.

		// Note: UpdateTask in service handles status updates too.
		updatedTask, err := s.UpdateTask(userCtx, taskID, project.UpdateTaskParams{
			Title:       req.Title,
			Description: req.Description,
			Status:      req.Status,
			AssigneeID:  req.AssigneeID,
		})

		if err != nil {
			// Handle Version Conflict?
			// The repo layer should ideally check versions, but for MVP we might force overwrite (Client Wins)
			// or fail. The domain interface allows returning specific conflict errors.
			return uuid.Nil, 0, err
		}

		// Calculate simple version based on time
		return updatedTask.ID, int(updatedTask.UpdatedAt.Unix()), nil

	default:
		return uuid.Nil, 0, fmt.Errorf("unsupported action: %s", op.Action)
	}
}

// FetchDelta returns tasks assigned to the user or in their projects.
func (s *Service) FetchDelta(ctx context.Context, actorID uuid.UUID, since time.Time) ([]sync_domain.EntityState, error) {
	// 1. Get User's Projects
	// We list all projects the user is a member of.
	projects, _, err := s.ListProjects(ctx, project.ProjectFilter{
		Limit: 100, // Reasonable cap for sync
	})
	if err != nil {
		return nil, err
	}

	results := make([]sync_domain.EntityState, 0)

	for _, p := range projects {
		// 2. Fetch Tasks for each Project
		// In a real optimized system, we'd use a specialized query `ListTasksModifiedAfter(since, memberID)`
		// spanning multiple projects to avoid N+1 queries.
		// For MVP, we iterate.
		tasks, _, err := s.repo.ListTasks(ctx, p.ID, project.TaskFilter{Limit: 500})
		if err != nil {
			continue
		}

		for _, t := range tasks {
			if t.UpdatedAt.After(since) {
				data, err := json.Marshal(t)
				if err != nil {
					s.logger.Error("failed to marshal task for sync", err, log.String("task_id", t.ID.String()))
					continue
				}
				results = append(results, sync_domain.EntityState{
					ID:        t.ID,
					Version:   int(t.UpdatedAt.Unix()),
					Deleted:   false, // Soft delete logic needed for true sync
					Data:      data,
					UpdatedAt: t.UpdatedAt,
				})
			}
		}
	}

	return results, nil
}

// =============================================================================
// HELPERS
// =============================================================================

func (s *Service) checkAccess(ctx context.Context, projectID uuid.UUID, perm core.PermissionName) error {
	actor := auth.ActorFromContext(ctx)
	if actor == nil {
		return auth.ErrUnauthorized
	}

	member, err := s.repo.GetMember(ctx, projectID, actor.UserID)
	if err != nil {
		if err == project.ErrMemberNotFound {
			return project.ErrAccessDenied
		}
		return err
	}

	allowed := false
	switch perm {
	case project.PermProjectRead:
		allowed = true
	case project.PermProjectUpdate, project.PermProjectTaskCreate, project.PermProjectTaskUpdate, project.PermProjectFileUpload:
		allowed = member.Role == project.RoleAdmin || member.Role == project.RoleMember
	case project.PermProjectDelete, project.PermProjectManageMembers, project.PermProjectFileDelete, project.PermProjectTaskDelete:
		allowed = member.Role == project.RoleAdmin
	}

	if !allowed {
		return project.ErrAccessDenied
	}
	return nil
}

func (s *Service) logActivity(ctx context.Context, repo project.Repository, projectID, actorID uuid.UUID, aType project.ActivityType, targetType *string, targetID *uuid.UUID, meta map[string]any) error {
	act := &project.Activity{
		ID:         uuid.New(),
		ProjectID:  projectID,
		ActorID:    actorID,
		Type:       aType,
		TargetType: targetType,
		TargetID:   targetID,
		Metadata:   meta,
		CreatedAt:  time.Now().UTC(),
	}
	return repo.CreateActivity(ctx, act)
}

func (s *Service) audit(ctx context.Context, actorID uuid.UUID, action core.EventAction, targetID uuid.UUID, msg string) {
	event := audit.NewEvent(actorID, "user", action).
		WithTarget(targetID, "project", msg).
		WithSuccess().
		WithContext(ctx).Build()
	if err := s.auditRecorder.Record(context.Background(), event); err != nil {
		s.logger.Error("failed to record audit event", err)
	}
}

func (s *Service) publishEvent(ctx context.Context, action core.EventAction, actor *auth.Actor, targetID uuid.UUID, targetType string, payload any) error {
	evt, _ := events.NewEvent(action, actor.UserID, "user", payload)
	evt.TargetID = &targetID
	evt.TargetType = &targetType
	return s.eventBus.Publish(ctx, evt)
}
