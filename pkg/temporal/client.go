package temporal

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/combi/pkg/config"
	"github.com/combi/pkg/log"
	"go.temporal.io/sdk/client"
)

// NewClient creates a new Temporal client with proper configuration and retry logic.
func NewClient(cfg config.TemporalConfig, logger log.Logger) (client.Client, error) {
	logger.Info("Initializing Temporal client",
		log.String("host_port", cfg.HostPort),
		log.String("namespace", cfg.Namespace))

	clientOptions := client.Options{
		HostPort:  cfg.HostPort,
		Namespace: cfg.Namespace,
		Logger:    newTemporalLogger(logger),
	}

	// Add TLS configuration if needed
	if cfg.TLS != nil && cfg.TLS.Enabled {
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}

		if cfg.TLS.CertPath != "" && cfg.TLS.KeyPath != "" {
			cert, err := tls.LoadX509KeyPair(cfg.TLS.CertPath, cfg.TLS.KeyPath)
			if err != nil {
				return nil, fmt.Errorf("failed to load TLS certificate: %w", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}

		clientOptions.ConnectionOptions = client.ConnectionOptions{
			TLS: tlsConfig,
		}
	}

	maxRetries := 30
	retryDelay := 3 * time.Second

	var c client.Client
	var err error

	for i := 0; i < maxRetries; i++ {
		c, err = client.Dial(clientOptions)

		if err == nil {
			logger.Info("Temporal client initialized successfully")
			return c, nil
		}

		if i < maxRetries-1 {
			logger.Warn("Failed to connect to Temporal, retrying...",
				log.Int("attempt", i+1),
				log.Int("max_retries", maxRetries),
				log.String("error", err.Error()))
			time.Sleep(retryDelay)
		}
	}

	return nil, fmt.Errorf("failed to create Temporal client after %d attempts: %w", maxRetries, err)
}
