package fxcore

import (
	"context"
	"fmt"

	"go.uber.org/fx"
	"github.com/combi/pkg/config"
	"github.com/combi/pkg/cache"
	pPostgres "github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/events"
	pEvents "github.com/combi/pkg/events/nats"
	pHealth "github.com/combi/pkg/health"
	pGrpc "github.com/combi/pkg/grpc"
	"github.com/combi/pkg/limiter"
	"github.com/combi/pkg/log"
	zlog "github.com/combi/pkg/log/zap"
	pMoney "github.com/combi/pkg/money"
	"github.com/combi/pkg/monitoring"
	pRedis "github.com/combi/pkg/redis"
	"github.com/combi/pkg/security"
	pTemporal "github.com/combi/pkg/temporal"
	"github.com/combi/pkg/trace"

	"github.com/jmoiron/sqlx"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	tClient "go.temporal.io/sdk/client"
)

// InfraModule bundles the core infrastructure dependencies WITHOUT the gRPC
// server and WITHOUT Temporal. Hosts that do not serve gRPC (e.g. the
// workflow-engine, which only runs Temporal workers, or worker-only hosts)
// should mount this instead of Module and add the providers they need.
//
// Note: ProvideTemporal is NOT included here either — Temporal connectivity
// is opt-in because NewClient retries for ~90s when the server is
// unreachable, which would stall host startup. Hosts that need Temporal add
// fx.Provide(fxcore.ProvideTemporal) explicitly.
var InfraModule = fx.Options(
	fx.Provide(
		ProvideLogger,
		ProvideTracer,
		ProvideMonitor,
		ProvideDatabase,
		ProvideRedis,
		ProvideNatsConnection,
		ProvideHealthChecker,
		ProvidePlatformUtils,
		ProvideEventBus,
	),
)

// Module bundles all core infrastructure dependencies.
var Module = fx.Options(
	InfraModule,
	fx.Provide(
		ProvideTemporal,
		ProvideGrpcServer,
	),
)

func ProvideLogger(cfg *config.Config, lc fx.Lifecycle) (log.Logger, error) {
	zapLogger, err := zlog.NewZapLogger(cfg.Log, cfg.Env)
	if err != nil {
		return nil, fmt.Errorf("failed to init logger: %w", err)
	}
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error { return zapLogger.Sync() },
	})
	return zlog.New(zapLogger), nil
}

func ProvideTracer(cfg *config.Config, lc fx.Lifecycle) (*trace.Provider, error) {
	provider, err := trace.NewProvider(trace.ProviderConfig{
		ServiceName: cfg.Trace.ServiceName,
		Environment: cfg.Env,
		Endpoint:    cfg.Trace.Endpoint,
		Disabled:    !cfg.Trace.Enabled,
	})
	if err != nil {
		return nil, err
	}
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error { return provider.Shutdown(ctx) },
	})
	return provider, nil
}

func ProvideMonitor(cfg *config.Config) monitoring.Service {
	return monitoring.NewRecorder(cfg.Trace.ServiceName)
}

func ProvideDatabase(cfg *config.Config, logger log.Logger, lc fx.Lifecycle) (*sqlx.DB, error) {
	db, err := pPostgres.NewConnection(cfg.Database, logger)
	if err != nil {
		return nil, err
	}
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error { return db.Close() },
	})
	return db, nil
}

func ProvideRedis(cfg *config.Config, logger log.Logger, lc fx.Lifecycle) (*redis.Client, error) {
	client := pRedis.NewClient(cfg.Redis, logger)
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error { return client.Close() },
	})
	return client, nil
}

func ProvideTemporal(cfg *config.Config, logger log.Logger, lc fx.Lifecycle) (tClient.Client, error) {
	client, err := pTemporal.NewClient(cfg.Temporal, logger)
	if err != nil {
		return nil, err
	}
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			client.Close()
			return nil
		},
	})
	return client, nil
}

func ProvideHealthChecker(db *sqlx.DB, redisClient *redis.Client) *pHealth.Checker {
	hc := pHealth.NewChecker()
	hc.Register("database", func(ctx context.Context) error { return db.PingContext(ctx) })
	hc.Register("redis", func(ctx context.Context) error { return redisClient.Ping(ctx).Err() })
	return hc
}

type PlatformUtils struct {
	fx.Out

	Cache           cache.Service
	Locker          cache.Locker
	IdemStore       cache.IdempotencyStore
	Random          security.RandomService
	Password        security.PasswordService
	DefaultCurrency pMoney.Currency
	Limiter         limiter.RateLimiter
}

func ProvidePlatformUtils(cfg *config.Config, redisClient *redis.Client, logger log.Logger) PlatformUtils {
	rateLimits, err := limiter.ParseRateLimits(cfg.RL.Limits)
	if err != nil {
		logger.Warn("Failed to parse rate limits", log.ErrorAttr(err))
	}

	// Africa-first default: the configured default currency wins; NGN is
	// the fallback because Nigeria is the primary launch market
	// (foundation spec). The launch-market stopgap set in pkg/money keeps
	// GetCurrency honest until F2's unified registry lands.
	defaultCurrencyCode := cfg.Localization.DefaultCurrency
	if defaultCurrencyCode == "" {
		defaultCurrencyCode = "NGN"
	}

	return PlatformUtils{
		Cache:           cache.NewRedisService(redisClient, "cache:"),
		Locker:          cache.NewRedisLocker(redisClient, "lock:"),
		IdemStore:       cache.NewRedisIdempotencyStore(redisClient, "idem:", logger),
		Random:          security.NewRandomService(),
		Password:        security.NewPasswordService(cfg.Auth.PasswordPolicy),
		DefaultCurrency: pMoney.GetCurrency(defaultCurrencyCode),
		Limiter:         limiter.NewRedisRateLimiter(redisClient, rateLimits, limiter.DefaultRate, "rl:", logger),
	}
}

func ProvideNatsConnection(cfg *config.Config, lc fx.Lifecycle) (*nats.Conn, error) {
	url := cfg.Nats.URL
	if url == "" {
		url = nats.DefaultURL
	}
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			nc.Close()
			return nil
		},
	})
	return nc, nil
}

// ProvideEventBus provides the system event bus implementation.
func ProvideEventBus(cfg *config.Config, nc *nats.Conn, idem cache.IdempotencyStore, logger log.Logger, monitor monitoring.Service, hc *pHealth.Checker, lc fx.Lifecycle) (events.Bus, error) {
	bus, err := pEvents.NewNatsJetStreamBus(
		nc,
		idem,
		logger,
		monitor,
		cfg.Trace.ServiceName,
		cfg.EventBus,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to init NATS JetStream event bus: %w", err)
	}
	
	hc.Register("event_bus", func(ctx context.Context) error {
		return bus.Health(ctx)
	})

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return bus.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return bus.Shutdown(ctx)
		},
	})
	
	return bus, nil
}

// ProvideGrpcServer provides the chassis gRPC server: standard interceptor
// chain, grpc_health_v1 (bridged to the platform health checker so the
// status flips NOT_SERVING while database/redis/event bus is down) and
// reflection — all mounted by pkg/grpc.NewServer.
func ProvideGrpcServer(cfg *config.Config, hc *pHealth.Checker, logger log.Logger, lc fx.Lifecycle) *pGrpc.Server {
	server := pGrpc.NewServer(cfg, logger,
		pGrpc.WithHealthChecker(hc, pGrpc.DefaultHealthPollInterval),
	)
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return server.Start()
		},
		OnStop: func(ctx context.Context) error {
			server.Stop()
			return nil
		},
	})
	return server
}
