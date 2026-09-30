package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/messaging"
	"github.com/combi/pkg/domain/sync_domain"
	"github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/events"
	"github.com/combi/pkg/log"
)

// Ensure implementation
var _ messaging.Service = (*Service)(nil)
var _ sync_domain.SyncableService = (*Service)(nil)

type Service struct {
	repo     messaging.Repository
	userSvc  user.Service // To fetch Names/Avatars for enrichment
	eventBus events.Bus
	logger   log.Logger
}

func NewService(
	repo messaging.Repository,
	userSvc user.Service,
	eventBus events.Bus,
	logger log.Logger,
) *Service {
	return &Service{
		repo:     repo,
		userSvc:  userSvc,
		eventBus: eventBus,
		logger:   logger.With(log.String("service", "messaging")),
	}
}

// --- Channel Management ---

func (s *Service) CreateDirectChannel(ctx context.Context, userA, userB uuid.UUID) (*messaging.Channel, error) {
	// 1. Check if exists
	if existing, err := s.repo.FindDirectChannel(ctx, userA, userB); err == nil {
		return existing, nil
	}

	// 2. Create new
	ch := &messaging.Channel{
		ID:   uuid.New(),
		Type: messaging.TypeDirect,
	}

	// Transactional creation
	// Note: WithTx needs to be exposed or we call methods individually.
	// For simplicity in this example, we assume sequential calls (minor race condition possible but low risk for 1-on-1 creation).
	if err := s.repo.CreateChannel(ctx, ch); err != nil {
		return nil, err
	}

	// 3. Add Members
	_ = s.repo.AddMember(ctx, &messaging.ChannelMember{ChannelID: ch.ID, UserID: userA, Role: "member"})
	_ = s.repo.AddMember(ctx, &messaging.ChannelMember{ChannelID: ch.ID, UserID: userB, Role: "member"})

	return ch, nil
}

func (s *Service) CreateGroupChannel(ctx context.Context, name string, creatorID uuid.UUID, initialMembers []uuid.UUID) (*messaging.Channel, error) {
	ch := &messaging.Channel{
		ID:   uuid.New(),
		Type: messaging.TypeGroup,
		Name: &name,
	}

	if err := s.repo.CreateChannel(ctx, ch); err != nil {
		return nil, err
	}

	// Add Creator as Admin
	_ = s.repo.AddMember(ctx, &messaging.ChannelMember{ChannelID: ch.ID, UserID: creatorID, Role: "admin"})

	// Add Members
	for _, uid := range initialMembers {
		if uid != creatorID {
			_ = s.repo.AddMember(ctx, &messaging.ChannelMember{ChannelID: ch.ID, UserID: uid, Role: "member"})
		}
	}

	return ch, nil
}

func (s *Service) CreateContextChannel(ctx context.Context, name string, refType string, refID uuid.UUID, creatorID uuid.UUID) (*messaging.Channel, error) {
	var cType messaging.ChannelType
	switch refType {
	case "project":
		cType = messaging.TypeProject
	case "event":
		cType = messaging.TypeEvent
	default:
		cType = messaging.TypeGroup
	}

	ch := &messaging.Channel{
		ID:      uuid.New(),
		Type:    cType,
		Name:    &name,
		RefType: &refType,
		RefID:   &refID,
	}

	if err := s.repo.CreateChannel(ctx, ch); err != nil {
		return nil, err
	}

	_ = s.repo.AddMember(ctx, &messaging.ChannelMember{ChannelID: ch.ID, UserID: creatorID, Role: "admin"})

	return ch, nil
}

func (s *Service) GetChannel(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*messaging.Channel, error) {
	// Verify Membership
	if _, err := s.repo.GetMember(ctx, id, userID); err != nil {
		return nil, messaging.ErrNotMember
	}
	return s.repo.GetChannel(ctx, id)
}

func (s *Service) ListUserChannels(ctx context.Context, userID uuid.UUID, limit, offset int) ([]messaging.ChannelSummary, error) {
	summaries, err := s.repo.ListChannelsForUser(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}

	for i, ch := range summaries {
		if ch.Type == messaging.TypeDirect {
			// Find the other member
			members, _ := s.repo.GetChannelMembers(ctx, ch.ID)
			var otherID uuid.UUID
			for _, mID := range members {
				if mID != userID {
					otherID = mID
					break
				}
			}
			// Fetch profile
			if otherID != uuid.Nil {
				if u, err := s.userSvc.GetProfile(ctx, otherID); err == nil {
					name := u.DisplayName // Assuming DisplayName is string
					summaries[i].Name = &name

					// Fix: AvatarFileID is typically a UUID or string in User Profile.
					// If it's a UUID, .String() returns string. We need *string.
					if u.AvatarFileID != nil {
						url := u.AvatarFileID.String()
						summaries[i].AvatarURL = &url
					}
				}
			}
		}
	}

	return summaries, nil
}

// --- Messaging ---

func (s *Service) SendMessage(ctx context.Context, channelID, senderID uuid.UUID, content string, mType messaging.MessageType, meta map[string]any) (*messaging.Message, error) {
	if content == "" && mType == messaging.MsgText {
		return nil, messaging.ErrEmptyMessage
	}

	// 1. Verify Membership
	if _, err := s.repo.GetMember(ctx, channelID, senderID); err != nil {
		return nil, messaging.ErrNotMember
	}

	// 2. Create Message
	metaBytes, _ := json.Marshal(meta)
	msg := &messaging.Message{
		ID:        uuid.New(),
		ChannelID: channelID,
		SenderID:  senderID,
		Type:      mType,
		Content:   content,
		Metadata:  metaBytes,
	}

	if err := s.repo.CreateMessage(ctx, msg); err != nil {
		return nil, err
	}

	// 3. Emit Realtime Event
	// RealtimeService will pick this up and push to WebSocket
	// NotificationService will pick this up and push Push/Email if users are offline
	evtPayload := map[string]any{
		"message_id": msg.ID,
		"channel_id": channelID,
		"sender_id":  senderID,
		"content":    content, // Truncate if huge?
		"type":       mType,
		"created_at": msg.CreatedAt,
	}

	if evt, err := events.NewEvent(core.EventMessageCreated, senderID, "user", evtPayload); err == nil {
		// Set Target to Channel ID for routing
		targetID := channelID
		targetType := "channel"
		evt.TargetID = &targetID
		evt.TargetType = &targetType

		_ = s.eventBus.Publish(ctx, evt)
	}

	return msg, nil
}

func (s *Service) ListMessages(ctx context.Context, channelID uuid.UUID, beforeID *uuid.UUID, limit int) ([]messaging.Message, error) {
	// Access control handled at handler level typically, or we check membership here?
	// Checking membership is safer.
	// However, fetching "sender" actor from context here is hard without changing interface.
	// Assuming Handler checks "Can user X access Channel Y".

	return s.repo.ListMessages(ctx, channelID, beforeID, limit)
}

func (s *Service) MarkRead(ctx context.Context, channelID, userID uuid.UUID) error {
	if err := s.repo.UpdateLastRead(ctx, channelID, userID, time.Now().UTC()); err != nil {
		return err
	}
	// Optional: Emit event for "Read Receipt" UI updates
	return nil
}

// --- Membership ---

func (s *Service) JoinChannel(ctx context.Context, channelID, userID uuid.UUID) error {
	// Check if public/joinable logic exists? Assuming yes or invited.
	return s.repo.AddMember(ctx, &messaging.ChannelMember{ChannelID: channelID, UserID: userID, Role: "member"})
}

func (s *Service) LeaveChannel(ctx context.Context, channelID, userID uuid.UUID) error {
	return s.repo.RemoveMember(ctx, channelID, userID)
}

// --- Moderation ---

func (s *Service) DeleteMessage(ctx context.Context, messageID, actorID uuid.UUID) error {
	msg, err := s.repo.GetMessage(ctx, messageID)
	if err != nil {
		return err
	}

	// Only Sender or Admin can delete
	if msg.SenderID != actorID {
		// Check if actor is admin of the channel
		member, err := s.repo.GetMember(ctx, msg.ChannelID, actorID)
		if err != nil || (member.Role != "admin" && member.Role != "owner") {
			return errors.New("forbidden")
		}
	}

	return s.repo.SoftDeleteMessage(ctx, messageID)
}

func (s *Service) FlagMessage(ctx context.Context, messageID, reporterID uuid.UUID, reason string) error {
	// 1. Emit Event for Moderation Service / Admin Dashboard
	payload := map[string]any{
		"message_id":  messageID,
		"reporter_id": reporterID,
		"reason":      reason,
	}
	evt, _ := events.NewEvent(core.EventMessageFlagged, reporterID, "user", payload)
	return s.eventBus.Publish(ctx, evt)
}

// --- Sync Implementation ---

// ApplySyncChange handles offline message sending.
func (s *Service) ApplySyncChange(ctx context.Context, actorID uuid.UUID, op sync_domain.SyncOperation) (uuid.UUID, int, error) {
	if op.EntityType != "message" {
		return uuid.Nil, 0, fmt.Errorf("messaging service only supports syncing 'message' entities, got: %s", op.EntityType)
	}

	if op.Action != sync_domain.ActionCreate {
		return uuid.Nil, 0, fmt.Errorf("offline message updates/deletes not supported in MVP")
	}

	// 1. Unmarshal Payload
	var req struct {
		ChannelID uuid.UUID             `json:"channel_id"`
		Content   string                `json:"content"`
		Type      messaging.MessageType `json:"type"`
		Metadata  map[string]any        `json:"metadata"`
	}
	if err := json.Unmarshal(op.Payload, &req); err != nil {
		return uuid.Nil, 0, fmt.Errorf("invalid payload: %w", err)
	}

	// 2. Call Logic
	// SendMessage handles validation (membership check) and persistence
	// NOTE: SendMessage emits an event which might trigger a push notification to self if not careful,
	// but usually realtime/notification service filters out the sender.
	msg, err := s.SendMessage(ctx, req.ChannelID, actorID, req.Content, req.Type, req.Metadata)
	if err != nil {
		return uuid.Nil, 0, err
	}

	// 3. Return Result
	// Version is typically 1 for new messages
	return msg.ID, 1, nil
}

// FetchDelta returns recent messages in active channels since 'since'.
func (s *Service) FetchDelta(ctx context.Context, actorID uuid.UUID, since time.Time) ([]sync_domain.EntityState, error) {
	// 1. Get User's Active Channels
	// We need to know which channels to check.
	// Assume ListChannelsForUser returns IDs of channels the user is in.
	channels, err := s.repo.ListChannelsForUser(ctx, actorID, 100, 0)
	if err != nil {
		return nil, err
	}

	results := make([]sync_domain.EntityState, 0)

	// 2. For each channel, fetch messages newer than 'since'
	// Optimization: Use a dedicated repo method `GetMessagesSince(userID, since)`
	// that joins memberships and messages to avoid N+1 queries.
	// For MVP, we iterate (inefficient but works).

	for _, ch := range channels {
		// We use ListMessages logic but filter by time in memory or specialized repo call.
		// Assuming repo has ListMessagesSince(channelID, since)
		// If not, we might fetch recent 50 and filter.
		msgs, err := s.repo.ListMessagesSince(ctx, ch.ID, since)
		// Fallback if ListMessagesSince doesn't exist: use ListMessages and filter
		if err != nil {
			// Fallback logic if method missing:
			msgs, _ = s.repo.ListMessages(ctx, ch.ID, nil, 50)
		}

		for _, m := range msgs {
			if m.CreatedAt.After(since) {
				data, _ := json.Marshal(m)
				results = append(results, sync_domain.EntityState{
					ID:        m.ID,
					Version:   int(m.CreatedAt.Unix()), // Immutable messages, version = creation time
					Deleted:   m.DeletedAt != nil,
					Data:      data,
					UpdatedAt: m.CreatedAt,
				})
			}
		}
	}

	return results, nil
}
