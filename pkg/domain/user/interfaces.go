package user

import (
	"context"
	"fmt"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/privacy"
	"github.com/google/uuid"
)

// Service defines the core user-facing contract for managing profiles and relationships.
type Service interface {
	privacy.Anonymizer
	// CreateUser is special: it's typically called by the AuthService, not directly by an actor.
	CreateUser(ctx context.Context, params CreateUserParams) (*User, error)
	GetUser(ctx context.Context, userID uuid.UUID) (*User, error)
	GetProfile(ctx context.Context, userID uuid.UUID) (*Profile, error)
	UpdateProfile(ctx context.Context, actorID, targetUserID uuid.UUID, params UpdateProfileParams) (*Profile, error)
	SetAvatar(ctx context.Context, targetUserID, fileID uuid.UUID) error

	GetPublicProfile(ctx context.Context, userID uuid.UUID) (*PublicProfile, error)
	GetPublicProfiles(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]PublicProfile, error)
	GetPublicProfileByEmail(ctx context.Context, email string) (*PublicProfile, error)
	GetContactInfo(ctx context.Context, userID uuid.UUID) (*ContactInfo, error)

	GetUserLocale(ctx context.Context, userID uuid.UUID) (string, error)

	// Connection Management
	CreateConnection(ctx context.Context, actorID, targetUserID uuid.UUID, connType ConnectionType) (*Connection, error)
	DeleteConnection(ctx context.Context, actorID, targetUserID uuid.UUID, connType ConnectionType) error

	// ListConnections returns users that the userID follows (Source -> Target).
	ListConnections(ctx context.Context, userID uuid.UUID, connType ConnectionType) ([]PublicProfile, error)

	// ListFollowers returns users that follow the userID (Target <- Source).
	ListFollowers(ctx context.Context, userID uuid.UUID, connType ConnectionType) ([]PublicProfile, error)
}

// AdminService defines administrative functions for user management.
type AdminService interface {
	ListUsers(ctx context.Context, filter ListUsersFilter) ([]*User, int, error)
	GetUserDetails(ctx context.Context, userID uuid.UUID) (*UserWithProfile, error)
	UpdateUser(ctx context.Context, userID uuid.UUID, params UpdateUserParams) (*User, error)
	UpdateUserStatus(ctx context.Context, userID uuid.UUID, status UserStatus, reason *string) (*User, error)
}

// --- DTOs & Params ---

type CreateUserParams struct {
	UserID    uuid.UUID
	Email     core.Email
	FirstName string
	LastName  string
}

type UpdateProfileParams struct {
	DisplayName  *string
	Bio          *string
	CustomFields map[string]interface{}
}

type UpdateUserParams struct {
	FirstName *string
	LastName  *string
	Email     *core.Email
}

type ListUsersFilter struct {
	Search string
	Status *UserStatus
	Limit  int
	Offset int
}

// UserWithProfile combines the core User and Profile models for detailed views.
type UserWithProfile struct {
	*User
	*Profile
}

type ContactInfo struct {
	Email       core.Email
	PhoneNumber string
}

// FullName provides a displayable name, preferring DisplayName over FirstName/LastName.
func (s *PublicProfile) FullName() string {
	if s.DisplayName != nil && *s.DisplayName != "" {
		return *s.DisplayName
	}
	return fmt.Sprintf("%s %s", s.FirstName, s.LastName)
}
