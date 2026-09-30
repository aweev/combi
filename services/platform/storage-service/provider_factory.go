package storage

import (
	"context"
	"fmt"
	"sync"

	"github.com/combi/pkg/config"
	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/log"
	"github.com/combi/storage/providers/cloudinary"
	"github.com/combi/storage/providers/local"
	"github.com/combi/storage/providers/s3"
)

// providerFactory is a concrete implementation of the storage.ProviderFactory interface.
type providerFactory struct {
	cfg           *config.Config
	repo          storage.Repository
	logger        log.Logger
	providerCache sync.Map
}

func NewProviderFactory(cfg *config.Config, repo storage.Repository, logger log.Logger) storage.ProviderFactory {
	return &providerFactory{
		cfg:    cfg,
		repo:   repo,
		logger: logger.With(log.String("component", "storage_provider_factory")),
	}
}

func (f *providerFactory) GetProvider(ctx context.Context, bucket *storage.StorageBucket) (storage.Provider, error) {
	// 1. Check Cache
	if instance, ok := f.providerCache.Load(bucket.ID); ok {
		return instance.(storage.Provider), nil
	}

	// 2. Instantiate
	f.logger.Info("Instantiating new storage provider", log.String("bucket_name", bucket.Name), log.String("provider_type", string(bucket.Provider)))
	instance, err := f.instantiateProvider(bucket)
	if err != nil {
		f.logger.Error("Failed to instantiate storage provider", err, log.String("bucket_id", bucket.ID.String()))
		return nil, err
	}

	// 3. Cache and Return
	f.providerCache.Store(bucket.ID, instance)
	return instance, nil
}

func (f *providerFactory) GetDefaultProvider(ctx context.Context) (storage.Provider, *storage.StorageBucket, error) {
	bucket, err := f.repo.GetDefaultBucket(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get default bucket: %w", err)
	}
	provider, err := f.GetProvider(ctx, bucket)
	return provider, bucket, err
}

func (f *providerFactory) instantiateProvider(bucket *storage.StorageBucket) (storage.Provider, error) {
	if !bucket.IsActive {
		return nil, fmt.Errorf("cannot instantiate provider for inactive bucket '%s'", bucket.Name)
	}

	switch bucket.Provider {
	case storage.ProviderS3:
		providerCfg := f.cfg.Storage.Providers.S3
		cfg := s3.Config{
			Region:    providerCfg.Region,
			AccessKey: providerCfg.AccessKey,
			SecretKey: providerCfg.SecretKey,
			Endpoint:  providerCfg.Endpoint,
		}
		return s3.New(cfg, f.logger)

	case storage.ProviderLocal:
		cfg := local.Config{
			BaseDir: f.cfg.Storage.Providers.Local.BaseDir,
		}
		return local.New(cfg, f.logger)

	case storage.ProviderCloudinary:
		providerCfg := f.cfg.Storage.Providers.Cloudinary
		cfg := cloudinary.Config{
			CloudName:      providerCfg.CloudName,
			APIKey:         providerCfg.APIKey,
			APISecret:      providerCfg.APISecret,
			UsePrivateURLs: providerCfg.UsePrivateURLs,
		}
		return cloudinary.New(cfg, f.logger)

	default:
		return nil, fmt.Errorf("%w: '%s'", storage.ErrProviderNotSupported, bucket.Provider)
	}
}
