// combi/cmd/platform-server — consolidated host binary for ALL platform
// services (per plan.md's binary consolidation strategy).
//
// Instead of 22+ individual service binaries, every platform service is an
// fx library module mounted here (see combi/cmd/service-init to generate
// one). The host provides: config, infrastructure (fxcore), the gRPC
// transport, and the mount point below.
//
// Run from the repo root (where config.yml lives):
//
//	go run ./cmd/platform-server
//
// Config resolution: config.Load walks "." / ./config / ../config, so the
// binary also works from subdirectories; CONFIG_PATH overrides the search
// root. NGO_-prefixed env vars override individual values.
package main

import (
	"fmt"
	"os"

	"go.uber.org/fx"

	acl "github.com/combi/acl"
	auditlog "github.com/combi/auditlog"
	auth "github.com/combi/auth"
	consentprivacy "github.com/combi/consent"
	identity "github.com/combi/identity"
	usersvc "github.com/combi/identity/user"
	integration "github.com/combi/integration"
	notification "github.com/combi/notification"
	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/audit"
	domainauth "github.com/combi/pkg/domain/auth"
	domainnotification "github.com/combi/pkg/domain/notification"
	domainprivacy "github.com/combi/pkg/domain/privacy"
	domainstorage "github.com/combi/pkg/domain/storage"
	domainuser "github.com/combi/pkg/domain/user"
	"github.com/combi/pkg/fxcore"
	pGrpc "github.com/combi/pkg/grpc"
	"github.com/combi/pkg/log"

	localization "github.com/combi/localization"
	messaging "github.com/combi/messaging"
	payments "github.com/combi/payments"
	secretsconfig "github.com/combi/config"
	storagesvc "github.com/combi/storage"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "platform-server: FATAL: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// app.Run blocks until SIGINT/SIGTERM (fx handles signals) and runs
	// graceful shutdown through the lifecycle hooks.
	fx.New(appOptions(cfg)...).Run()
	return nil
}

func loadConfig() (*config.Config, error) {
	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		path = "."
	}
	return config.Load(path)
}

// appOptions assembles the platform-server fx graph. Kept separate from
// run() so tests can validate the exact production graph via
// fx.ValidateApp without any infrastructure running.
func appOptions(cfg *config.Config) []fx.Option {
	return []fx.Option{
		fx.Supply(cfg),

		// Host = infra (config, logger, db, redis, nats, event bus,
		// health, platform utils) + gRPC transport. Temporal is
		// intentionally NOT wired here: workflow-engine owns Temporal,
		// and the client retries ~90s against a missing server. Add
		// fx.Provide(fxcore.ProvideTemporal) if a mounted service
		// genuinely needs to start workflows in-process.
		fxcore.InfraModule,
		fx.Provide(fxcore.ProvideGrpcServer),

		// ── Service modules mount here as they are ported ──────────
		// Each services/platform/<name>-service module exposes Module.
		// In-process service calls resolve the module's Service
		// interface from this graph; cross-process calls go through
		// pkg/grpc.DialService instead (see pkg/grpc/client.go).
		acl.Module,
		auditlog.Module,
		auth.Module,
		consentprivacy.Module,
		identity.Module,
		integration.Module,
		localization.Module,
		messaging.Module,
		notification.Module,
		payments.Module,
		secretsconfig.Module,
		storagesvc.Module,

		fx.Invoke(mountPlatformServer, bindAuditRecorder, bindExportNotifier, bindExportStorage, bindPrivacyManager, bindIdentityAuth, bindAuthUser),
	}
}

// bindAuditRecorder closes the loop left open by the RBAC port: acl's
// AdminService late-binds its audit recorder via SetAuditRecorder, and the
// audit module mounted in this host provides the implementation (the async
// event recorder). Every RBAC mutation is audit-logged from boot on.
func bindAuditRecorder(recorder audit.EventRecorder, admin *acl.AdminService, logger log.Logger) {
	admin.SetAuditRecorder(recorder)
	logger.Info("audit recorder bound to RBAC admin service")
}

// bindExportNotifier closes the loop left open by the notification port:
// auditlog's ExportWorker cannot constructor-wire notification.Service
// (notification consumes audit.EventRecorder — wiring both directions
// cycles the fx graph), so the host late-binds it here, mirroring api's
// "Injected later" wiring in build_services.go. Export completion alerts
// flow from boot on.
func bindExportNotifier(ns domainnotification.Service, worker *auditlog.ExportWorker, logger log.Logger) {
	worker.SetNotificationService(ns)
	logger.Info("notification service bound to audit export worker")
}

// bindExportStorage closes the loop left open by the storage port:
// auditlog's ExportWorker cannot constructor-wire storage.Service (storage
// hard-requires audit.EventRecorder — same cycle shape as the notification
// port), so the host late-binds it here, mirroring api's
// InjectStorageService wiring in build_services.go.
func bindExportStorage(ss domainstorage.Service, worker *auditlog.ExportWorker, logger log.Logger) {
	worker.SetStorageService(ss)
	logger.Info("storage service bound to audit export worker")
}
// bindPrivacyManager closes the loop left open by the consent-privacy port:
// auditlog's ComplianceService cannot constructor-wire privacy.Manager
// (consent-privacy consumes audit.EventRecorder — same cycle shape as the
// notification port), so the host late-binds it here, mirroring api's
// InjectPrivacyManager wiring in build_services.go.
func bindPrivacyManager(pm domainprivacy.Manager, compliance *auditlog.ComplianceService, logger log.Logger) {
	compliance.SetPrivacyManager(pm)
	logger.Info("privacy manager bound to audit compliance service")
}

// bindIdentityAuth closes the user→auth direction: the identity AdminService
// needs auth.AdminService for account email/status/session flows, but taking
// it as an fx edge would cycle with auth (which consumes user.Service).
// Late-bound here, mirroring api's post-construction injection.
func bindIdentityAuth(adminSvc domainauth.AdminService, userContainer *usersvc.Container, logger log.Logger) {
	userContainer.AdminService.SetAuthAdminService(adminSvc)
	logger.Info("auth admin service bound to identity admin service")
}

// bindAuthUser closes the auth→user direction: auth's Service and
// OAuthService resolve user profiles via user.Service (naked in standalone
// mode, where auth guards the nil except in the OAuth registration flow).
// Both directions are bound at host boot, so the full graph is live.
func bindAuthUser(us domainuser.Service, as domainauth.Service, os domainauth.OAuthService, logger log.Logger) {
	as.SetUserService(us)
	os.SetUserService(us)
	logger.Info("user service bound to auth service and OAuth service")
}

// mountPlatformServer logs the boot line. The gRPC health protocol,
// reflection, and the standard interceptor chain are mounted by the
// chassis itself (pkg/grpc.NewServer via fxcore.ProvideGrpcServer) —
// hosts no longer hand-register them.
func mountPlatformServer(srv *pGrpc.Server, cfg *config.Config, logger log.Logger) {
	logger.Info("platform-server ready",
		log.Int("grpc_port", srv.Port()),
		log.String("env", cfg.Env))
}
