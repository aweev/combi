package user

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	WithTx(ctx context.Context, fn func(txRepo Repository) error) error

	UserRepository
	ProfileRepository
	ConnectionRepository
}

type UserRepository interface {
	CreateUser(ctx context.Context, user *User) error
	GetUser(ctx context.Context, userID uuid.UUID) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	ListUsers(ctx context.Context, filter ListUsersFilter) ([]*User, int, error)
	UpdateUser(ctx context.Context, user *User) error
	AnonymizeUser(ctx context.Context, userID uuid.UUID) error
	DeletePendingUsers(ctx context.Context, olderThan time.Time) (int64, error)
	GetPublicProfiles(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]PublicProfile, error)
}

type ProfileRepository interface {
	CreateProfile(ctx context.Context, profile *Profile) error
	GetProfile(ctx context.Context, userID uuid.UUID) (*Profile, error)
	UpdateProfile(ctx context.Context, profile *Profile) error
}

type ConnectionRepository interface {
	CreateConnection(ctx context.Context, conn *Connection) error
	GetConnection(ctx context.Context, sourceUserID, targetUserID uuid.UUID, connType ConnectionType) (*Connection, error)
	DeleteConnection(ctx context.Context, id uuid.UUID) error
	ListConnectionTargetIDs(ctx context.Context, sourceUserID uuid.UUID, connType ConnectionType) ([]uuid.UUID, error)
	ListConnections(ctx context.Context, sourceUserID uuid.UUID, connType ConnectionType) ([]*Connection, error)
	// ListConnectionSourceIDs finds users who have connected TO the target (e.g. followers).
	ListConnectionSourceIDs(ctx context.Context, targetUserID uuid.UUID, connType ConnectionType) ([]uuid.UUID, error)
}
