Event Bus Implementation Guide
Namespace: platform.events
Tier: 0 (Core Infrastructure)
The central nervous system of the the NGO Platform platform. It provides asynchronous, decoupled communication between services using Redis Streams. It guarantees at-least-once delivery and handles consumer group coordination automatically.

1. Overview
The Event Bus allows services to react to changes in other services without tight coupling.
Feature	Implementation
Transport	Redis Streams: Persistent, ordered log of events.
Scaling	Consumer Groups: Multiple instances of a service automatically share the load.
Reliability	Idempotency: Prevents duplicate event processing using a dedicated Redis Set + TTL.
Recovery	Reaper: A background process that "reaps" events from consumers that crashed while processing.
Observability	OpenTelemetry: Every publish/consume action starts or continues a distributed trace.

Representation

┌──────────────┐      ┌────────────────────┐      ┌────────────────┐
│  Publisher   │ ───> │    Redis Stream    │ <─── │ Consumer Group │
└──────────────┘      │  (Persistent Log)  │      └───────┬────────┘
                      └────────────────────┘              │
                                                          ▼
                                                  ┌────────────────┐
                                                  │    Consumer    │
                                                  │   (Service A)  │
                                                  └───────┬────────┘
                                                          │
                                      ┌───────────────────┼───────────────────┐
                                      ▼                   ▼                   ▼
                                [Idempotency]        [Logic Exec]           [Ack]
2. Configuration
Controlled via config.yml under the eventBus key.
Key	Description	Default
channelPrefix	Prefix for Redis keys to avoid collisions.	the NGO Platform:events:
maxRetries	Attempts before moving to DLQ.	3
reaperPollInterval	How often to check for stuck messages.	1m
reaperGracePeriod	Time a message can be "processing" before being reclaimed.	30s

3. Go SDK Usage
Publishing an Event
Fire-and-forget. The bus handles serialization and tracing.

// 1. Define Payload
payload := map[string]string{
    "user_id": "uuid-1234",
    "email":   "user@example.com",
}

// 2. Create Domain Event
evt, _ := events.NewEvent(
    core.EventAction("user.created"), // Topic
    actorID,                          // Who did it
    "user",                           // Actor Type
    payload,                          // Data
)

// 3. Publish
err := deps.EventBus.Publish(ctx, evt)
Subscribing to an Event
Register handlers during service initialization (app.go).

// In internal/services/email/worker.go

func (w *EmailWorker) RegisterSubscribers(bus events.Bus) {
    bus.Subscribe("user.created", w.HandleUserCreated)
}

func (w *EmailWorker) HandleUserCreated(ctx context.Context, evt events.DomainEvent) error {
    var payload UserCreatedPayload
    if err := evt.UnmarshalPayload(&payload); err != nil {
        return err // Will be retried or DLQ'd
    }

    // Business Logic...
    return w.mailer.SendWelcomeEmail(ctx, payload.Email)
}
4. Operational Runbooks
Handling "Poison Pill" Messages (DLQ)
If a message fails processing maxRetries times, it is moved to a Dead Letter Queue stream (suffixed with :dlq).
Inspect DLQ:
redis-cli XREAD COUNT 10 STREAMS the NGO Platform:events:user.created:dlq 0
Fix the Bug: Deploy a code fix for the consumer.
Replay: Write a script to move messages from :dlq back to the main stream the NGO Platform:events:user.created.
Monitoring
Metric: the NGO Platform_events_published_total (Counter)
Metric: the NGO Platform_events_processed_total (Counter, labelled by status=success|failure)
Metric: the NGO Platform_job_duration_seconds (Histogram for handler latency)
If processed_total < published_total for an extended time, consumers are lagging. Scale up the service pods.
