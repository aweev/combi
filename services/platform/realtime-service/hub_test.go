package realtime

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/combi/pkg/config"
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/realtime"
	rt_mocks "github.com/combi/pkg/domain/realtime/mocks"
	"github.com/combi/pkg/log"
	metrics_mocks "github.com/combi/pkg/monitoring/mocks"
)

// mockConnection is a helper to spy on what the Hub writes to a client
type mockConnection struct {
	mu       sync.Mutex
	messages []interface{}
}

func (m *mockConnection) WriteJSON(v any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, v)
	return nil
}
func (m *mockConnection) ReadPump(hub realtime.Hub, client *realtime.Client) {} // No-op for this test
func (m *mockConnection) WritePump(client *realtime.Client) {
	// Simulate the write pump logic: read from Send channel, write to conn
	for msg := range client.Send {
		_ = m.WriteJSON(msg)
	}
}
func (m *mockConnection) Close() error { return nil }

func TestHub_Whitebox(t *testing.T) {
	// Mocks
	mockBroker := rt_mocks.NewMockBroker(t)
	mockAuthz := rt_mocks.NewMockSubscriptionAuthorizer(t)
	mockAudit := audit_mocks.NewMockEventRecorder(t)
	mockMetrics := metrics_mocks.NewMockRecorder(t)

	// Broker Setup
	brokerChan := make(chan *realtime.Message)
	mockBroker.EXPECT().Subscribe(mock.Anything).Return((<-chan *realtime.Message)(brokerChan), nil)
	mockBroker.EXPECT().Close().Return(nil)

	// Initialize Hub
	hub := NewHub(
		config.RealtimeConfig{},
		mockBroker,
		mockAuthz,
		mockAudit,
		mockMetrics,
		&log.NoOpLogger{},
	).(*Hub) // Cast to concrete struct to access internals if needed

	// Start Hub in background
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = hub.Run(ctx)
	}()

	// Teardown
	defer func() {
		cancel()
		wg.Wait()
	}()

	t.Run("Client Registration and Subscription Flow", func(t *testing.T) {
		userID := uuid.New()
		channel := realtime.ChannelID("public:test")

		// 1. Create Fake Client
		conn := &mockConnection{}
		client := &realtime.Client{
			ID:            uuid.New(),
			UserID:        &userID,
			Subscriptions: make(map[realtime.ChannelID]struct{}),
			Conn:          conn,
			Send:          make(chan *realtime.Message, 10),
		}

		// Start the fake write pump
		go conn.WritePump(client)

		// 2. Register Client (Simulate HandleConnection)
		mockAudit.EXPECT().Record(mock.Anything, mock.MatchedBy(func(e interface{}) bool { return true })).Return(nil).Once() // Connect audit
		hub.register <- client

		// Wait for registration
		time.Sleep(10 * time.Millisecond)
		assert.Contains(t, hub.clients, client.ID)

		// 3. Client sends SUBSCRIBE command
		cmd := realtime.ClientCommand{
			Action:  "SUBSCRIBE",
			Channel: channel,
		}
		cmdBytes, _ := json.Marshal(cmd)

		// Mocks for Subscription
		mockAuthz.EXPECT().CanSubscribe(mock.Anything, &userID, channel).Return(true, nil).Once()
		mockBroker.EXPECT().Subscribe(mock.Anything, channel).Return(nil, nil).Once()
		mockAudit.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once() // Subscribe audit

		// Inject message directly into hub loop
		hub.ProcessClientMessage(client, cmdBytes)

		time.Sleep(50 * time.Millisecond)

		// Verify State
		assert.Contains(t, client.Subscriptions, channel)
		assert.Contains(t, hub.channels[channel], client.ID)

		// 4. Receive Message from Broker (Broadcast)
		msg := &realtime.Message{
			Channel: channel,
			Event:   "update",
			Payload: []byte(`{"val":1}`),
		}
		brokerChan <- msg // Inject from "Redis"

		time.Sleep(10 * time.Millisecond)

		// Verify Client Received it
		conn.mu.Lock()
		assert.Len(t, conn.messages, 1)
		received := conn.messages[0].(*realtime.Message)
		assert.Equal(t, "update", received.Event)
		conn.mu.Unlock()
	})
}
