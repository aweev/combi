package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/apperror"
	"github.com/google/uuid"
)

var (
	ErrUpgradeFailed      = apperror.New(http.StatusBadRequest, "UPGRADE_FAILED", "Could not upgrade connection to WebSocket.", nil)
	ErrInvalidPayload     = apperror.New(http.StatusBadRequest, "INVALID_PAYLOAD", "Message payload is invalid.", nil)
	ErrPublishFailed      = apperror.New(http.StatusInternalServerError, "PUBLISH_FAILED", "Failed to broadcast message.", nil)
	ErrSubscriptionDenied = apperror.New(http.StatusForbidden, "SUBSCRIPTION_DENIED", "Not authorized to subscribe to this channel.", nil)
)

const (
	// Emitted when a user establishes a WS connection.
	EventUserConnected core.EventAction = "realtime.user.connected"

	// Emitted when a user disconnects (useful for analytics/audit).
	EventUserDisconnected core.EventAction = "realtime.user.disconnected"

	// Emitted when a user subscribes to a sensitive channel.
	EventChannelSubscribed core.EventAction = "realtime.channel.subscribed"
)
const (
	// JobCleanupStaleConnections verifies that connections in the presence store
	// are actually alive, removing ghosts.
	JobCleanupStaleConnections core.JobType = "realtime:cleanup_stale_connections"
)

const (
	// PermRealtimePresenceRead allows viewing who is online.
	PermRealtimePresenceRead core.PermissionName = "realtime:presence:read"

	// PermRealtimeBroadcastSystem allows sending system-wide alerts.
	PermRealtimeBroadcastSystem core.PermissionName = "realtime:broadcast:system"
)

// ChannelID is a strongly typed string for channels (e.g., "public:impact", "user:123").
type ChannelID string

func (c ChannelID) String() string { return string(c) }

// IsUserChannel checks if the channel is private to a specific user.
func (c ChannelID) IsUserChannel() bool {
	return strings.HasPrefix(string(c), "user:")
}

// GetUserID extracts the UUID from a "user:{uuid}" channel.
func (c ChannelID) GetUserID() uuid.UUID {
	parts := strings.Split(string(c), ":")
	if len(parts) >= 2 && parts[0] == "user" {
		id, err := uuid.Parse(parts[1])
		if err == nil {
			return id
		}
	}
	return uuid.Nil
}

// Message is the payload sent over the wire.
type Message struct {
	Event     string          `json:"event"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp time.Time       `json:"timestamp"`
	Channel   ChannelID       `json:"channel,omitempty"` // Included for internal routing
}

// PublishRequest is the DTO for internal services to send messages.
type PublishRequest struct {
	Channel ChannelID `validate:"required"`
	Event   string    `validate:"required"`
	Data    any       `validate:"required"`
}

// Client represents a connected WebSocket user.
type Client struct {
	ID            uuid.UUID
	UserID        *uuid.UUID
	Subscriptions map[ChannelID]struct{}
	// The abstraction for the underlying websocket connection
	Conn Connection
	// Buffered channel of outbound messages
	Send chan *Message
}

// ClientCommand represents an incoming message from the client (e.g., Subscribe).
type ClientCommand struct {
	Action  string          `json:"action"` // "SUBSCRIBE", "UNSUBSCRIBE"
	Channel ChannelID       `json:"channel"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

const (
	PermRealtimeSubscribe core.PermissionName = "realtime:subscribe"

	// Audit/System Events
	EventClientConnected    core.EventAction = "realtime.client.connected"
	EventClientDisconnected core.EventAction = "realtime.client.disconnected"
	EventSubscription       core.EventAction = "realtime.subscription"
)

// Hub is the main entry point for the realtime service logic.
type Hub interface {
	// Run starts the event loop. Must be run in a goroutine.
	Run(ctx context.Context) error

	// HandleConnection upgrades an HTTP request to a WebSocket and registers it.
	HandleConnection(w http.ResponseWriter, r *http.Request)

	// Publish sends a message to a channel (distributing to all nodes).
	Publish(ctx context.Context, req PublishRequest) error

	// Internal methods used by the Connection adapter
	Unregister(client *Client)
	ProcessClientMessage(client *Client, message []byte)
}

// Broker handles multi-node message propagation (e.g., Redis Pub/Sub).
type Broker interface {
	Publish(ctx context.Context, channel ChannelID, message *Message) error
	Subscribe(ctx context.Context, channels ...ChannelID) (<-chan *Message, error)
	Unsubscribe(ctx context.Context, channels ...ChannelID) error
	Close() error
}

// SubscriptionAuthorizer determines if a specific user can listen to a specific channel.
type SubscriptionAuthorizer interface {
	CanSubscribe(ctx context.Context, userID *uuid.UUID, channel ChannelID) (bool, error)
}

// Connection abstracts the WebSocket library (e.g., Gorilla).
type Connection interface {
	// WritePump pumps messages from the Hub to the websocket connection.
	WritePump(client *Client)
	// ReadPump pumps messages from the websocket connection to the Hub.
	ReadPump(hub Hub, client *Client)

	Close() error
	WriteJSON(v any) error
}

type PresenceRepository interface {
	// Heartbeat refreshes a user's online status.
	Heartbeat(ctx context.Context, userID uuid.UUID, clientID uuid.UUID, ttl time.Duration) error

	// GetOnlineUsers returns a list of user IDs currently connected to the specific channel (or global).
	GetOnlineUsers(ctx context.Context, channel ChannelID) ([]uuid.UUID, error)

	// RemoveClient removes a specific connection.
	RemoveClient(ctx context.Context, userID uuid.UUID, clientID uuid.UUID) error
}
