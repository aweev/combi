package messaging

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Service is the primary interface used by other domains.
type Service interface {
	// Channel Management
	CreateDirectChannel(ctx context.Context, userA, userB uuid.UUID) (*Channel, error)
	CreateGroupChannel(ctx context.Context, name string, creatorID uuid.UUID, initialMembers []uuid.UUID) (*Channel, error)
	CreateContextChannel(ctx context.Context, name string, refType string, refID uuid.UUID, creatorID uuid.UUID) (*Channel, error)
	GetChannel(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*Channel, error)
	ListUserChannels(ctx context.Context, userID uuid.UUID, limit, offset int) ([]ChannelSummary, error)

	// Membership
	JoinChannel(ctx context.Context, channelID, userID uuid.UUID) error
	LeaveChannel(ctx context.Context, channelID, userID uuid.UUID) error

	// Messaging
	SendMessage(ctx context.Context, channelID, senderID uuid.UUID, content string, mType MessageType, meta map[string]any) (*Message, error)
	ListMessages(ctx context.Context, channelID uuid.UUID, beforeID *uuid.UUID, limit int) ([]Message, error)
	MarkRead(ctx context.Context, channelID, userID uuid.UUID) error

	// Moderation
	FlagMessage(ctx context.Context, messageID, reporterID uuid.UUID, reason string) error
	DeleteMessage(ctx context.Context, messageID, actorID uuid.UUID) error
}

// Repository defines the persistence contract.
type Repository interface {
	// Channels
	CreateChannel(ctx context.Context, c *Channel) error
	GetChannel(ctx context.Context, id uuid.UUID) (*Channel, error)
	FindDirectChannel(ctx context.Context, userA, userB uuid.UUID) (*Channel, error)
	ListChannelsForUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]ChannelSummary, error)
	UpdateChannelTimestamp(ctx context.Context, id uuid.UUID, t time.Time) error

	// Members
	AddMember(ctx context.Context, cm *ChannelMember) error
	RemoveMember(ctx context.Context, channelID, userID uuid.UUID) error
	GetMember(ctx context.Context, channelID, userID uuid.UUID) (*ChannelMember, error)
	UpdateLastRead(ctx context.Context, channelID, userID uuid.UUID, t time.Time) error
	GetChannelMembers(ctx context.Context, channelID uuid.UUID) ([]uuid.UUID, error)

	// Messages
	CreateMessage(ctx context.Context, m *Message) error
	GetMessage(ctx context.Context, id uuid.UUID) (*Message, error)
	ListMessages(ctx context.Context, channelID uuid.UUID, beforeID *uuid.UUID, limit int) ([]Message, error)
	ListMessagesSince(ctx context.Context, channelID uuid.UUID, since time.Time) ([]Message, error)
	SoftDeleteMessage(ctx context.Context, id uuid.UUID) error
}
