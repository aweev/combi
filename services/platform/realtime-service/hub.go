package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/audit"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/core"
	"github.com/combi/pkg/domain/realtime"
	"github.com/combi/pkg/domain/system"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/monitoring"
	platformRT "github.com/combi/pkg/realtime"
)

type Hub struct {
	config     config.RealtimeConfig
	broker     realtime.Broker
	authorizer realtime.SubscriptionAuthorizer
	audit      audit.EventRecorder
	logger     log.Logger

	clients  map[uuid.UUID]*realtime.Client
	channels map[realtime.ChannelID]map[uuid.UUID]bool
	metrics  monitoring.Recorder

	register       chan *realtime.Client
	unregister     chan *realtime.Client
	broadcast      chan *realtime.Message
	clientMessages chan clientMessage
}

type clientMessage struct {
	client  *realtime.Client
	message []byte
}

func NewHub(
	cfg config.RealtimeConfig,
	broker realtime.Broker,
	authz realtime.SubscriptionAuthorizer,
	auditRec audit.EventRecorder,
	metrics monitoring.Recorder,
	logger log.Logger,
) realtime.Hub {
	return &Hub{
		config:         cfg,
		broker:         broker,
		authorizer:     authz,
		audit:          auditRec,
		metrics:        metrics,
		logger:         logger.With(log.String("service", "realtime_hub")),
		clients:        make(map[uuid.UUID]*realtime.Client),
		channels:       make(map[realtime.ChannelID]map[uuid.UUID]bool),
		register:       make(chan *realtime.Client),
		unregister:     make(chan *realtime.Client),
		broadcast:      make(chan *realtime.Message),
		clientMessages: make(chan clientMessage),
	}
}

func (h *Hub) Run(ctx context.Context) error {
	h.logger.Info("Realtime Hub started")

	// Subscribe returns a channel that receives messages from Redis
	brokerMsgs, err := h.broker.Subscribe(ctx)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			h.broker.Close()
			return nil

		case client := <-h.register:
			h.clients[client.ID] = client
			h.recordAudit(ctx, client, realtime.EventClientConnected, audit.OutcomeSuccess, nil)

		case client := <-h.unregister:
			if _, ok := h.clients[client.ID]; ok {
				for ch := range client.Subscriptions {
					h.unsubscribeClient(ctx, client, ch)
				}
				delete(h.clients, client.ID)
				close(client.Send)
				h.recordAudit(ctx, client, realtime.EventClientDisconnected, audit.OutcomeSuccess, nil)
			}

		case cm := <-h.clientMessages:
			h.handleClientCommand(ctx, cm.client, cm.message)

		case msg := <-h.broadcast:
			// Outbound: App -> Broker -> Redis
			if err := h.broker.Publish(ctx, msg.Channel, msg); err != nil {
				h.logger.Error("failed to publish message to broker", err, log.String("channel", string(msg.Channel)))
			}

		case msg, ok := <-brokerMsgs:
			if !ok {
				return fmt.Errorf("broker channel closed")
			}
			// Inbound: Redis -> Broker -> App -> Clients
			h.dispatch(msg)
		}
	}
}

func (h *Hub) HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := platformRT.NewGorillaConnection(w, r, h.config.AllowedOrigins)
	if err != nil {
		h.logger.Error("Websocket upgrade failed", err)
		return
	}

	var userID *uuid.UUID
	actor := auth.ActorFromContext(r.Context())
	if actor != nil {
		userID = &actor.AccountID
	}

	client := &realtime.Client{
		ID:            uuid.New(),
		UserID:        userID,
		Subscriptions: make(map[realtime.ChannelID]struct{}),
		Conn:          conn,
		Send:          make(chan *realtime.Message, 256),
	}

	h.register <- client

	go conn.WritePump(client)
	go conn.ReadPump(h, client)
}

func (h *Hub) Publish(ctx context.Context, req realtime.PublishRequest) error {
	payload, _ := json.Marshal(req.Data)
	msg := &realtime.Message{
		Event:     req.Event,
		Payload:   payload,
		Timestamp: time.Now().UTC(),
		Channel:   req.Channel,
	}

	select {
	case h.broadcast <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Internal Helpers
func (h *Hub) dispatch(msg *realtime.Message) {
	recipients := h.channels[msg.Channel]
	if len(recipients) == 0 {
		return
	}
	for clientID := range recipients {
		if client, ok := h.clients[clientID]; ok {
			select {
			case client.Send <- msg:
				// Success
			default:
				h.logger.Warn("Client buffer full, dropping message",
					log.String("client_id", clientID.String()),
					log.String("channel", string(msg.Channel)),
				)
				// Record Metric
				h.metrics.IncrementCounter(monitoring.MetricMessagesDropped, monitoring.MetricLabels{
					"channel_type": getChannelType(msg.Channel),
				})
			}
		}
	}
}

func getChannelType(ch realtime.ChannelID) string {
	if ch.IsUserChannel() {
		return "user"
	}
	return "topic"
}

func (h *Hub) handleClientCommand(ctx context.Context, client *realtime.Client, msg []byte) {
	var cmd realtime.ClientCommand
	if err := json.Unmarshal(msg, &cmd); err != nil {
		return
	}

	switch cmd.Action {
	case "SUBSCRIBE":
		if allowed, _ := h.authorizer.CanSubscribe(ctx, client.UserID, cmd.Channel); allowed {
			h.subscribeClient(ctx, client, cmd.Channel)
		} else {
			// Send error back to client
			errPayload, _ := json.Marshal(map[string]string{"error": "Forbidden", "channel": string(cmd.Channel)})
			client.Send <- &realtime.Message{Event: "error", Payload: errPayload}
		}
	case "UNSUBSCRIBE":
		h.unsubscribeClient(ctx, client, cmd.Channel)

	case "SIGNAL":
		// User sends an SDP offer/answer or ICE candidate.
		// We rebroadcast this to the channel so the other peer(s) receive it.
		// Security: Must be subscribed to the channel to signal on it.
		if _, subscribed := client.Subscriptions[cmd.Channel]; subscribed {
			// Construct the message to broadcast
			// We wrap the client's payload.
			// Ideally, we add "sender_id" to the payload so the receiver knows who it's from.

			// Simple approach: Republish as a standard message
			msg := &realtime.Message{
				Event:     "signal", // Clients listen for "signal"
				Channel:   cmd.Channel,
				Payload:   cmd.Payload, // The SDP/ICE JSON
				Timestamp: time.Now().UTC(),
			}
			// Use broadcast channel (goes to Redis -> All Nodes -> Clients)
			h.broadcast <- msg
		} else {
			// Optional: Send error "Not subscribed"
		}
	}
}

func (h *Hub) subscribeClient(ctx context.Context, client *realtime.Client, ch realtime.ChannelID) {
	if _, ok := h.channels[ch]; !ok {
		h.channels[ch] = make(map[uuid.UUID]bool)
		if _, err := h.broker.Subscribe(ctx, ch); err != nil {
			h.logger.Error("failed to subscribe to channel on broker", err, log.String("channel", string(ch)))
		}
	}
	h.channels[ch][client.ID] = true
	client.Subscriptions[ch] = struct{}{}
	h.recordAudit(ctx, client, realtime.EventSubscription, audit.OutcomeSuccess, map[string]interface{}{"channel": ch, "action": "subscribe"})
}

func (h *Hub) unsubscribeClient(ctx context.Context, client *realtime.Client, ch realtime.ChannelID) {
	if _, ok := h.channels[ch]; ok {
		delete(h.channels[ch], client.ID)
		delete(client.Subscriptions, ch)
		if len(h.channels[ch]) == 0 {
			delete(h.channels, ch)
			if err := h.broker.Unsubscribe(ctx, ch); err != nil {
				h.logger.Error("failed to unsubscribe from channel on broker", err, log.String("channel", string(ch)))
			}
		}
	}
}

// Interface Glue
func (h *Hub) Unregister(c *realtime.Client) { h.unregister <- c }
func (h *Hub) ProcessClientMessage(c *realtime.Client, m []byte) {
	select {
	case h.clientMessages <- clientMessage{c, m}:
	default:
	}
}

func (h *Hub) recordAudit(ctx context.Context, c *realtime.Client, action core.EventAction, outcome audit.Outcome, meta map[string]any) {
	actorID := system.SystemActorID
	actorType := system.SystemActorType
	if c.UserID != nil {
		actorID = *c.UserID
		actorType = "user"
	}

	event := audit.NewEvent(actorID, actorType, action).
		WithOutcome(outcome).
		WithMetadata(meta).
		WithContext(ctx).
		Build()

	// Fire and forget audit
	go func() {
		if err := h.audit.Record(context.Background(), event); err != nil {
			h.logger.Warn("failed to record realtime audit event", log.ErrorAttr(err))
		}
	}()
}
