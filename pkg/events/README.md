# pkg/events — domain event bus

**Transport: NATS JetStream.** Mounted by the host at `pkg/fxcore/module.go:202`
(`ProvideEventBus` → `pEvents.NewNatsJetStreamBus`), per `docs/01-DECISIONS.md` **D-07**.
Subject = `eventBus.channelPrefix + <topic>` (e.g. `combi:events:user.created`); one stream is
created/updated lazily with subjects `combi:events:*`.

> **History.** This file previously documented the bus as **Redis Streams**, with `redis-cli` DLQ
> runbooks and `the NGO Platform` key prefixes. That described `pkg/events/redis/`, not the mounted
> implementation. Anyone following those instructions was debugging a system that is not running.

## What this package contains

| Path | Role |
|---|---|
| `types.go`, `emit.go` | `events.Bus` contract, `DomainEvent`, `events.Emit` (fills request/tenant/trace metadata from context) |
| `nats/bus.go` | **the implementation in use** — JetStream publisher/consumer |
| `redis/` | Redis Streams bus + `reaper.go`. **Not mounted** (D-07). Kept for `pkg/queue`, which legitimately uses Redis Streams for *jobs* |
| `mocks/` | mockery-generated `Bus` mock |

## Delivery semantics (as implemented)

- **At-least-once.** The consumer uses `AckExplicitPolicy`; a message is acked only when every handler
  registered for the topic returns nil. On failure it is `Nak`'d and JetStream redelivers.
- **Idempotency.** `cache.IdempotencyStore` guards each delivery with key `evt:<event-id>:<service-name>`,
  stored 24h after success; duplicate deliveries are acked and skipped. Consumers must still be idempotent —
  the guard is skipped when no store is injected.
- **Unparseable payloads are acked and dropped**, since redelivery could never succeed.
- **Tracing.** OpenTelemetry spans around publish and consume; trace context propagates via `event.Metadata`.

## Known gaps — verified in code, not assumed from config

`config.yml` carries `dlqSuffix`, `maxRetries`, `reaperPollInterval`, `reaperGracePeriod`.
**The JetStream bus reads only `channelPrefix`; the other four do not affect its behaviour.**

1. **No dead-letter subject exists.** A permanently failing handler is Nak'd and redelivered forever — the
   consumer sets no `MaxDeliver`. Poison messages need either `MaxDeliver` + a real DLQ subject, or the
   `pkg/queue` job path, which *does* implement reaper and `:dlq`.
2. **No event metrics are emitted.** `bus.go` accepts and stores a `monitoring.Recorder` but never calls it
   (`b.monitor` appears zero times). `events_published_total` / `events_processed_total` are registered in
   `pkg/monitoring` and recorded **only by the unmounted Redis bus**. The published-vs-processed lag
   comparison therefore reads zero on the live system. Until fixed, use the NATS server's own stream/consumer
   stats and `event_bus` healthcheck as the signal.
3. **Idempotency-store errors `Nak` the message**, so a Redis outage becomes a redelivery loop rather than
   degraded-but-forwarding delivery.
4. **Fan-out is per-process.** `Subscribe` registers in-memory handlers, so two services sharing a topic
   name inside one host both receive it (durable consumer names include the service name).

## Usage

Publish (fire-and-forget; serialization and tracing are handled):

```go
evt, err := events.NewEvent(core.EventAction("user.created"), actorID, "user", payload)
if err != nil { return err }
err = bus.Publish(ctx, evt)   // never emit in order to request work — that is a job (D-08)
```

Subscribe during service construction, before `Start`:

```go
func (s *Service) RegisterSubscribers(bus events.Bus) {
    bus.Subscribe("user.created", s.HandleUserCreated)
}
```

`fxcore.InfraModule` provides the bus and registers a `"event_bus"` health check that fails when NATS is
not `CONNECTED`.

## Naming rule

`<domain>.<object>.<past-tense-action>` — `circle.member.joined`, `financial.subscription.created`.
Circle's required events are defined once in `docs/02-DOMAIN.md` §9; the metrics that consume them are in
`docs/06-MEASUREMENT.md`. Events describe facts that already happened; durable work goes through
`pkg/queue` or Temporal (D-08).
