// internal/persistence/postgres/scheduler_notifier.go
package postgres

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/log"
	"github.com/lib/pq"
)

const notificationChannel = "schedule_changes"

// Statically verify that it implements the domain interface.
var _ queue.ScheduleChangeNotifier = (*SchedulerNotifier)(nil)

// SchedulerNotifier listens for PostgreSQL notifications about schedule changes
// and forwards them to a Go channel. It manages its own dedicated database connection.
type SchedulerNotifier struct {
	logger        log.Logger
	dsn           string
	listener      *pq.Listener
	notifications chan string
	wg            sync.WaitGroup
	mu            sync.Mutex
	isListening   bool
	cancelFunc    context.CancelFunc
}

// NewSchedulerNotifier creates a new notifier. It requires the DSN to create its
// own dedicated connection for listening.
func NewSchedulerNotifier(dsn string, logger log.Logger) *SchedulerNotifier {
	return &SchedulerNotifier{
		dsn:           dsn,
		logger:        logger.With(log.String("component", "scheduler_notifier")),
		notifications: make(chan string, 10), // Buffered channel prevents blocking the driver
	}
}

// Listen starts the background goroutine that listens for database notifications.
// It is idempotent and safe to call multiple times.
func (n *SchedulerNotifier) Listen(ctx context.Context) error {
	n.mu.Lock()
	if n.isListening {
		n.mu.Unlock()
		return nil // Already listening
	}

	n.logger.Info("starting postgres notification listener", log.String("channel", notificationChannel))

	// Create a new listener with a custom connection reporting function.
	reportProblem := func(ev pq.ListenerEventType, err error) {
		if err != nil {
			n.logger.Error("postgres listener problem", err, log.String("event_type", fmt.Sprint(ev)))
		}
	}
	listener := pq.NewListener(n.dsn, 10*time.Second, time.Minute, reportProblem)

	if err := listener.Listen(notificationChannel); err != nil {
		_ = listener.Close()
		n.mu.Unlock()
		return fmt.Errorf("failed to start listening on channel %s: %w", notificationChannel, err)
	}

	n.listener = listener
	n.isListening = true

	// Create a cancellable context for our internal loop
	loopCtx, cancel := context.WithCancel(ctx)
	n.cancelFunc = cancel

	n.mu.Unlock()

	n.wg.Add(1)
	go n.runLoop(loopCtx)

	return nil
}

// runLoop is the core loop that waits for notifications and forwards them.
func (n *SchedulerNotifier) runLoop(ctx context.Context) {
	defer n.wg.Done()

	for {
		select {
		case <-ctx.Done():
			n.logger.Info("context canceled, shutting down notification listener loop")
			return
		case notification, ok := <-n.listener.Notify:
			if !ok {
				// This indicates the listener connection was permanently closed.
				n.logger.Warn("postgres notification channel was closed, loop terminating")
				return
			}
			if notification == nil {
				// This can happen on reconnects or timeouts. It's not an error.
				continue
			}

			n.logger.Debug("received notification", log.String("payload", notification.Extra))
			// Non-blocking send to the public channel
			select {
			case n.notifications <- notification.Extra:
			default:
				n.logger.Warn("notification channel is full, dropping notification payload", log.String("payload", notification.Extra))
			}
		case <-time.After(45 * time.Second):
			// Periodically ping the connection to keep it alive and check its health.
			// The go func() prevents blocking the select loop.
			go func() {
				if err := n.listener.Ping(); err != nil {
					n.logger.Error("failed to ping postgres listener connection", err)
				}
			}()
		}
	}
}

// Notifications returns the read-only channel where payloads are sent.
func (n *SchedulerNotifier) Notifications() <-chan string {
	return n.notifications
}

// Close gracefully shuts down the listener.
func (n *SchedulerNotifier) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.isListening {
		return nil
	}

	n.logger.Info("closing postgres notification listener")

	// Signal the runLoop to stop
	if n.cancelFunc != nil {
		n.cancelFunc()
	}

	err := n.listener.Close()
	n.isListening = false

	// Wait for the runLoop to finish
	n.wg.Wait()
	close(n.notifications)

	return err
}
