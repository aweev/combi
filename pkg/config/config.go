package config

import (
	"fmt"
	"time"

	"github.com/combi/pkg/i18n/locale"
)

// Environments recognized across hosts. Empty is treated as development
// (backwards compatibility with unpopulated test configs).
const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvStaging     = "staging"
	EnvProduction  = "production"
)

type Config struct {
	Env          string               `mapstructure:"env"`
	Server       ServerConfig         `mapstructure:"server"`
	Log          LogConfig            `mapstructure:"log"`
	Database     PostgresConfig       `mapstructure:"database"`
	Redis        RedisConfig          `mapstructure:"redis"`
	Monitoring   MonitoringConfig     `mapstructure:"monitoring"`
	Auth         AuthConfig           `mapstructure:"auth"`
	OAuth        OAuthConfig          `mapstructure:"oauth"`
	Queue        QueueConfig          `mapstructure:"queue"`
	EventBus     EventBusConfig       `mapstructure:"eventBus"`
	RL           RateLimiterConfig    `mapstructure:"rateLimiter"`
	CB           CircuitBreakerConfig `mapstructure:"circuitBreaker"`
	Trace        TraceConfig          `mapstructure:"trace"`
	Secrets      SecretsConfig        `mapstructure:"secrets"`
	Storage      StorageConfig        `mapstructure:"storage"`
	Localization LocalizationConfig   `mapstructure:"localization"`
	Features     FeatureFlagConfig    `mapstructure:"features"`
	Realtime     RealtimeConfig       `mapstructure:"realtime"`
	Messaging    MessagingConfig      `mapstructure:"messaging"`
	Temporal     TemporalConfig       `mapstructure:"temporal"`
	Email        EmailConfig          `mapstructure:"email"`
	Achievement  AchievementConfig    `mapstructure:"achievement"`
	Automation   AutomationConfig     `mapstructure:"automation"`
	Matchmaking  MatchmakingConfig    `mapstructure:"matchmaking"`
	Video        VideoConfig          `mapstructure:"video"`
	Nats         NatsConfig           `mapstructure:"nats"`
}

// Validate fails fast on malformed or contradictory configuration instead
// of letting services boot half-alive (chassis hardening, F1). It checks
// structural correctness only; production secret strength is enforced
// separately by the loader (validateProductionSecrets).
//
// Empty optional values are allowed — hosts apply their Africa-first
// defaults (e.g. NGN default currency, en-NG default locale) downstream.
func (c *Config) Validate() error {
	switch c.Env {
	case "", EnvDevelopment, EnvTest, EnvStaging, EnvProduction:
		// ok
	default:
		return fmt.Errorf("config: env %q is not one of development|test|staging|production", c.Env)
	}

	if p := c.Server.GRPCPort; p < 0 || p > 65535 {
		return fmt.Errorf("config: server.grpcPort %d out of range 1-65535", p)
	}

	if l := c.Localization.DefaultLocale; l != "" {
		if _, ok := locale.Lookup(locale.Locale(l)); !ok {
			return fmt.Errorf("config: localization.defaultLocale %q is not a supported locale (see pkg/i18n/locale)", l)
		}
	}

	if cur := c.Localization.DefaultCurrency; cur != "" && !isISO4217Code(cur) {
		return fmt.Errorf("config: localization.defaultCurrency %q must be a 3-letter uppercase ISO 4217 code", cur)
	}

	return nil
}

// isISO4217Code checks the structural shape of an ISO 4217 alphabetic code.
func isISO4217Code(code string) bool {
	if len(code) != 3 {
		return false
	}
	for i := 0; i < len(code); i++ {
		if code[i] < 'A' || code[i] > 'Z' {
			return false
		}
	}
	return true
}

type NatsConfig struct {
	URL string `mapstructure:"url"`
}

// --- SERVER ---
type ServerConfig struct {
	Port            int           `mapstructure:"port"`
	GRPCPort        int           `mapstructure:"grpcPort"`
	ShutdownTimeout time.Duration `mapstructure:"shutdownTimeout"`
	InternalSecret  string        `mapstructure:"internalSecret"`
	PublicDomain    string        `mapstructure:"publicDomain"`
	CORS            CORSConfig    `mapstructure:"cors"`
	HTTP            HTTPConfig    `mapstructure:"http"` // Make sure this exists
}

type HTTPConfig struct {
	ReadTimeout  time.Duration `mapstructure:"readTimeout"`
	WriteTimeout time.Duration `mapstructure:"writeTimeout"`
	IdleTimeout  time.Duration `mapstructure:"idleTimeout"`
}

type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"allowedOrigins"`
	AllowedMethods []string `mapstructure:"allowedMethods"`
	AllowedHeaders []string `mapstructure:"allowedHeaders"`
}

// --- LOGGING ---
type LogConfig struct {
	Level string `mapstructure:"level"`
}

// --- DATABASE ---
type PostgresConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"` // Env: NGO_DATABASE_PASSWORD
	DBName          string        `mapstructure:"dbname"`
	SSLMode         string        `mapstructure:"sslmode"`
	MaxOpenConns    int           `mapstructure:"maxOpenConns"`
	MaxIdleConns    int           `mapstructure:"maxIdleConns"`
	ConnMaxLifetime time.Duration `mapstructure:"connMaxLifetime"`
}

func (p PostgresConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		p.Host, p.Port, p.User, p.Password, p.DBName, p.SSLMode)
}

// --- REDIS ---
type RedisConfig struct {
	Host          string        `mapstructure:"host"`
	Port          int           `mapstructure:"port"`
	Password      string        `mapstructure:"password"` // Env: NGO_REDIS_PASSWORD
	DB            int           `mapstructure:"db"`
	PoolSize      int           `mapstructure:"poolSize"`
	MinIdleConns  int           `mapstructure:"minIdleConns"`
	MaxRetries    int           `mapstructure:"maxRetries"`
	DialTimeout   time.Duration `mapstructure:"dialTimeout"`
	ReadTimeout   time.Duration `mapstructure:"readTimeout"`
	WriteTimeout  time.Duration `mapstructure:"writeTimeout"`
	PoolTimeout   time.Duration `mapstructure:"poolTimeout"`
	EnableTracing bool          `mapstructure:"enableTracing"`
}

func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

type MonitoringConfig struct {
	PrometheusURL      string        `mapstructure:"prometheusURL"`      // e.g. http://prometheus:9090
	EvaluationInterval time.Duration `mapstructure:"evaluationInterval"` // e.g. 1m
}

// --- AUTH ---
type AuthConfig struct {
	PasswordPolicy PasswordPolicyConfig `mapstructure:"passwordPolicy"`
	JWT            JWTConfig            `mapstructure:"jwt"`
	MFA            MFAConfig            `mapstructure:"mfa"`
	// Env: NGO_AUTH_TOTPENCRYPTIONKEY
	TOTPEncryptionKey string `mapstructure:"totpEncryptionKey"`
	// Env: NGO_AUTH_AUDITENCRYPTIONKEY
	AuditEncryptionKey      string        `mapstructure:"auditEncryptionKey"`
	MaxFailedLogins         int           `mapstructure:"maxFailedLogins"`
	LockoutDuration         time.Duration `mapstructure:"lockoutDuration"`
	OAuthSuccessRedirectURL string        `mapstructure:"oauthSuccessRedirectURL"`
	Links                   LinkConfig    `mapstructure:"links"`
}

type LinkConfig struct {
	EmailVerificationURL string `mapstructure:"emailVerificationURL"`
	PasswordResetURL     string `mapstructure:"passwordResetURL"`
}

type PasswordPolicyConfig struct {
	MinLength           int  `mapstructure:"minLength"`
	RequireUppercase    bool `mapstructure:"requireUppercase"`
	RequireLowercase    bool `mapstructure:"requireLowercase"`
	RequireNumbers      bool `mapstructure:"requireNumbers"`
	RequireSpecialChars bool `mapstructure:"requireSpecialChars"`
}

type JWTConfig struct {
	Issuer        string        `mapstructure:"issuer"`
	AccessSecret  string        `mapstructure:"accessSecret"` // Env: NGO_AUTH_JWT_ACCESSSECRET
	RefreshSecret string        `mapstructure:"refreshSecret"`
	AccessTTL     time.Duration `mapstructure:"accessTTL"`
	RefreshTTL    time.Duration `mapstructure:"refreshTTL"`
}

type MFAConfig struct {
	TOTPIssuer string `mapstructure:"totpIssuer"`
}

// --- OAUTH ---
type OAuthConfig struct {
	Providers map[string]OAuthProviderConfig `mapstructure:"providers"`
}

type OAuthProviderConfig struct {
	ClientID     string   `mapstructure:"clientID"`     // Env: NGO_OAUTH_PROVIDERS_GOOGLE_CLIENTID
	ClientSecret string   `mapstructure:"clientSecret"` // Env: NGO_OAUTH_PROVIDERS_GOOGLE_CLIENTSECRET
	RedirectURL  string   `mapstructure:"redirectURL"`
	Scopes       []string `mapstructure:"scopes"`
}

// --- QUEUE ---
type QueueConfig struct {
	Worker          QueueWorkerConfig   `mapstructure:"worker"`
	Scheduler       SchedulerConfig     `mapstructure:"scheduler"`
	Reaper          ReaperConfig        `mapstructure:"reaper"`
	Outbox          OutboxConfig        `mapstructure:"outbox"`
	Deduplication   DeduplicationConfig `mapstructure:"deduplication"`
	WebhookSecret   string              `mapstructure:"webhookSecret"`
	JobRouting      map[string]string   `mapstructure:"jobRouting"`
	QueuePriorities map[string]int      `mapstructure:"queuePriorities"`
	Idempotency     IdempotencyConfig   `mapstructure:"idempotency"`
}

type QueueWorkerConfig struct {
	ID                string        `mapstructure:"id"`
	Host              string        `mapstructure:"host"`
	Concurrency       int           `mapstructure:"concurrency"`
	PollInterval      time.Duration `mapstructure:"pollInterval"`
	HeartbeatInterval time.Duration `mapstructure:"heartbeatInterval"`
}

type SchedulerConfig struct {
	PollInterval time.Duration `mapstructure:"pollInterval"`
	LockTTL      time.Duration `mapstructure:"lockTTL"`
}

type ReaperConfig struct {
	PollInterval  time.Duration `mapstructure:"pollInterval"`
	GracePeriod   time.Duration `mapstructure:"gracePeriod"`
	MaxDeliveries int           `mapstructure:"maxDeliveries"`
}

type OutboxConfig struct {
	PollInterval time.Duration `mapstructure:"pollInterval"`
	BatchSize    int           `mapstructure:"batchSize"`
}

type DeduplicationConfig struct {
	TTL       time.Duration `mapstructure:"ttl"`
	KeyPrefix string        `mapstructure:"keyPrefix"`
}

type IdempotencyConfig struct {
	LockTTL   time.Duration `mapstructure:"lockTTL"`
	ResultTTL time.Duration `mapstructure:"resultTTL"`
}

// --- EVENT BUS ---
type EventBusConfig struct {
	ChannelPrefix      string        `mapstructure:"channelPrefix"`
	DLQSuffix          string        `mapstructure:"dlqSuffix"`
	MaxRetries         int           `mapstructure:"maxRetries"`
	RetryDelay         time.Duration `mapstructure:"retryDelay"`
	ReaperPollInterval time.Duration `mapstructure:"reaperPollInterval"`
	ReaperGracePeriod  time.Duration `mapstructure:"reaperGracePeriod"`
}

// --- RESILIENCE ---
type RateLimiterConfig struct {
	Enabled bool              `mapstructure:"enabled"`
	Limits  map[string]string `mapstructure:"limits"`
}

type CircuitBreakerConfig struct {
	Enabled          bool          `mapstructure:"enabled"`
	Interval         time.Duration `mapstructure:"interval"`
	Timeout          time.Duration `mapstructure:"timeout"`
	FailureThreshold float64       `mapstructure:"failureThreshold"`
	SuccessThreshold uint32        `mapstructure:"successThreshold"`
}

// --- TRACE ---
type TraceConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	ServiceName string `mapstructure:"serviceName"`
	Endpoint    string `mapstructure:"endpoint"`
}

// --- SECRETS ---
type SecretsConfig struct {
	Provider string      `mapstructure:"provider"`
	Vault    VaultConfig `mapstructure:"vault"`
}

type VaultConfig struct {
	Address string        `mapstructure:"address"`
	Token   string        `mapstructure:"token"`
	Prefix  string        `mapstructure:"prefix"`
	AppRole AppRoleConfig `mapstructure:"approle"`
}

type AppRoleConfig struct {
	RoleID   string `mapstructure:"roleID"`   // Env: NGO_SECRETS_VAULT_APPROLE_ROLEID
	SecretID string `mapstructure:"secretID"` // Env: NGO_SECRETS_VAULT_APPROLE_SECRETID
}

// --- STORAGE ---
type StorageConfig struct {
	DefaultQuotaBytes   int64           `mapstructure:"defaultQuotaBytes"`
	UploadURLExpiry     time.Duration   `mapstructure:"uploadUrlExpiry"`
	DownloadURLExpiry   time.Duration   `mapstructure:"downloadUrlExpiry"`
	MaxUploadSizeBytes  int64           `mapstructure:"maxUploadSizeBytes"`
	ProcessingQueue     string          `mapstructure:"processingQueue"`
	AllowedMimePatterns []string        `mapstructure:"allowedMimePatterns"`
	Providers           ProviderConfigs `mapstructure:"providers"`
}

type ProviderConfigs struct {
	Local      LocalProviderConfig      `mapstructure:"local"`
	S3         S3ProviderConfig         `mapstructure:"s3"`
	Cloudinary CloudinaryProviderConfig `mapstructure:"cloudinary"`
}

type LocalProviderConfig struct {
	BaseDir string `mapstructure:"baseDir"`
}

type S3ProviderConfig struct {
	Region    string  `mapstructure:"region"`
	AccessKey string  `mapstructure:"accessKey"` // Env: NGO_STORAGE_PROVIDERS_S3_ACCESSKEY
	SecretKey string  `mapstructure:"secretKey"` // Env: NGO_STORAGE_PROVIDERS_S3_SECRETKEY
	Endpoint  *string `mapstructure:"endpoint"`
}

type CloudinaryProviderConfig struct {
	CloudName      string `mapstructure:"cloudName"`
	APIKey         string `mapstructure:"apiKey"`
	APISecret      string `mapstructure:"apiSecret"`
	UsePrivateURLs bool   `mapstructure:"usePrivateUrls"`
}

// --- MISC ---
type LocalizationConfig struct {
	DefaultLocale   string        `mapstructure:"defaultLocale"`
	DefaultCurrency string        `mapstructure:"defaultCurrency"`
	CacheTTL        time.Duration `mapstructure:"cacheTTL"`
}

type FeatureFlagConfig struct {
	FallbackPollInterval time.Duration `mapstructure:"fallbackPollInterval"`
}

type RealtimeConfig struct {
	AllowedOrigins []string `mapstructure:"allowedOrigins"`
}

type MessagingConfig struct {
	Enabled           bool          `mapstructure:"enabled"`
	MaxMessageLength  int           `mapstructure:"maxMessageLength"`  // e.g. 4096 chars
	MessageEditWindow time.Duration `mapstructure:"messageEditWindow"` // e.g. 5m
	DefaultListLimit  int           `mapstructure:"defaultListLimit"`  // e.g. 50
}

type TemporalConfig struct {
	HostPort  string             `mapstructure:"hostPort"`
	Namespace string             `mapstructure:"namespace"`
	TLS       *TemporalTLSConfig `mapstructure:"tls"`
}

type TemporalTLSConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	CertPath string `mapstructure:"certPath"`
	KeyPath  string `mapstructure:"keyPath"`
}

type EmailConfig struct {
	Default      string `mapstructure:"default"`
	ResendAPIKey string `mapstructure:"resendApiKey"`
}

type AchievementConfig struct {
	Enabled             bool          `mapstructure:"enabled"`
	LeaderboardCacheTTL time.Duration `mapstructure:"leaderboardCacheTTL"`
}

type AutomationConfig struct {
	RetentionDays       int           `mapstructure:"retentionDays"`
	RecoveryInterval    time.Duration `mapstructure:"recoveryInterval"`
	StatsFlushInterval  time.Duration `mapstructure:"statsFlushInterval"`
	AnonymizationSecret string        `mapstructure:"anonymizationSecret"`
}

type MatchmakingConfig struct {
	Enabled              bool          `mapstructure:"enabled"`
	AutoMatchInterval    time.Duration `mapstructure:"autoMatchInterval"`    // e.g. "24h"
	MinScoreThreshold    float64       `mapstructure:"minScoreThreshold"`    // e.g. 0.5
	NotifyScoreThreshold float64       `mapstructure:"notifyScoreThreshold"` // e.g. 0.8
}

type VideoConfig struct {
	Enabled    bool             `mapstructure:"enabled"`
	IceServers []IceServerConfig `mapstructure:"iceServers"`
}

type IceServerConfig struct {
	URLs       []string `mapstructure:"urls"`
	Username   string   `mapstructure:"username"`
	Credential string   `mapstructure:"credential"`
}
