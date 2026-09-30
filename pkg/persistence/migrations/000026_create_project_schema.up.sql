-- Auto-Generated Schema Isolation for project
CREATE SCHEMA IF NOT EXISTS project;
SET search_path TO project, public;

DROP TABLE IF EXISTS project_activities;
DROP TABLE IF EXISTS project_files;
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS milestones;
DROP TABLE IF EXISTS project_members;
DROP TABLE IF EXISTS projects;

-- 1. Projects
CREATE TABLE projects (
    id UUID PRIMARY KEY,
    org_id UUID, -- Nullable (Personal projects vs Org projects)
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    type VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL,
    creator_id UUID NOT NULL,
    
    start_date TIMESTAMPTZ,
    end_date TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Unique name per organization
CREATE UNIQUE INDEX idx_projects_org_name ON projects (org_id, name) WHERE org_id IS NOT NULL;
CREATE INDEX idx_projects_status ON projects(status);
CREATE INDEX idx_projects_creator ON projects(creator_id);

-- 2. Project Members
CREATE TABLE project_members (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    role VARCHAR(50) NOT NULL,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    PRIMARY KEY (project_id, user_id)
);

CREATE INDEX idx_project_members_user ON project_members(user_id);

-- 3. Milestones
CREATE TABLE milestones (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    due_date TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    is_critical BOOLEAN NOT NULL DEFAULT FALSE,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_milestones_project ON milestones(project_id);

-- 4. Tasks
CREATE TABLE tasks (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    milestone_id UUID REFERENCES milestones(id) ON DELETE SET NULL,
    
    title VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(50) NOT NULL,
    priority INTEGER NOT NULL DEFAULT 1,
    
    creator_id UUID NOT NULL,
    assignee_id UUID,
    due_date TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tasks_project ON tasks(project_id);
CREATE INDEX idx_tasks_milestone ON tasks(milestone_id);
CREATE INDEX idx_tasks_assignee ON tasks(assignee_id);
CREATE INDEX idx_tasks_status ON tasks(status);

-- 5. Project Files
CREATE TABLE project_files (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    file_id UUID NOT NULL, -- External Reference
    
    file_name VARCHAR(255) NOT NULL,
    file_type VARCHAR(100) NOT NULL,
    url TEXT NOT NULL,
    
    uploader_id UUID NOT NULL,
    description TEXT,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_project_files_project ON project_files(project_id);

-- 6. Activities (Timeline)
CREATE TABLE project_activities (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    actor_id UUID NOT NULL,
    type VARCHAR(100) NOT NULL,
    
    target_type VARCHAR(50),
    target_id UUID,
    metadata JSONB NOT NULL DEFAULT '{}',
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_project_activities_project ON project_activities(project_id);