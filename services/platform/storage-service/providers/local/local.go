// internal/services/storage/providers/local/local.go
package local

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/log"
)

// Config holds the configuration for the local file system provider.
type Config struct {
	BaseDir string // The root directory where files will be stored.
}

// Provider implements the storage.Provider interface for the local filesystem.
type Provider struct {
	cfg    Config
	logger log.Logger
}

// Statically assert that Provider implements the domain interface.
var _ storage.Provider = (*Provider)(nil)

// New creates a new local storage provider. It ensures the base directory exists.
func New(cfg Config, logger log.Logger) (storage.Provider, error) {
	if err := os.MkdirAll(cfg.BaseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create local storage base directory '%s': %w", cfg.BaseDir, err)
	}
	return &Provider{
		cfg:    cfg,
		logger: logger.With(log.String("provider", "local")),
	}, nil
}

// fullPath constructs the absolute local path for a file.
func (p *Provider) fullPath(bucket, path string) string {
	// For local storage, the "bucket" is just a top-level subdirectory.
	return filepath.Join(p.cfg.BaseDir, bucket, path)
}

// GeneratePresignedUploadURL is a placeholder for local development.
func (p *Provider) GeneratePresignedUploadURL(ctx context.Context, bucket, path, mimeType string, size int64, expiresIn time.Duration) (*storage.SignedURL, error) {
	// Local provider doesn't generate real signed URLs. This is for API compatibility.
	// A more advanced local setup might run a simple server to handle these "signed" requests.
	p.logger.Debug("Generated placeholder upload URL", log.String("path", path))
	return &storage.SignedURL{
		URL:       fmt.Sprintf("/dev/storage/upload/%s/%s", bucket, path),
		Method:    "PUT",
		ExpiresAt: time.Now().UTC().Add(expiresIn),
	}, nil
}

// GeneratePresignedDownloadURL is a placeholder for local development.
func (p *Provider) GeneratePresignedDownloadURL(ctx context.Context, bucket, path string, expiresIn time.Duration) (*storage.SignedURL, error) {
	p.logger.Debug("Generated placeholder download URL", log.String("path", path))
	return &storage.SignedURL{
		URL:       fmt.Sprintf("/dev/storage/download/%s/%s", bucket, path),
		Method:    "GET",
		ExpiresAt: time.Now().UTC().Add(expiresIn),
	}, nil
}

func (p *Provider) Upload(ctx context.Context, bucket, path string, reader io.Reader, size int64, mimeType string) error {
	fullPath := p.fullPath(bucket, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for local upload: %w", err)
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create local file for upload: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	if err != nil {
		return fmt.Errorf("failed to copy data to local file: %w", err)
	}
	return nil
}

func (p *Provider) Download(ctx context.Context, bucket, path string) (io.ReadCloser, error) {
	file, err := os.Open(p.fullPath(bucket, path))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, storage.ErrFileNotFound
		}
		return nil, err
	}
	return file, nil
}

func (p *Provider) Delete(ctx context.Context, bucket, path string) error {
	err := os.Remove(p.fullPath(bucket, path))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil // Idempotent: return nil if it's already gone.
}

func (p *Provider) Exists(ctx context.Context, bucket, path string) (bool, error) {
	_, err := os.Stat(p.fullPath(bucket, path))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
