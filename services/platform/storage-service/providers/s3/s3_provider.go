// internal/services/storage/providers/s3/s3.go
package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/log"
)

// Config holds the necessary configuration for connecting to an S3 bucket.
type Config struct {
	Region    string
	AccessKey string
	SecretKey string
	Endpoint  *string // For S3-compatible services like MinIO
}

// Provider implements the storage.Provider interface for AWS S3 and compatible services.
type Provider struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	uploader      *manager.Uploader
	logger        log.Logger
}

// Statically assert that Provider implements the domain interface.
var _ storage.Provider = (*Provider)(nil)

// New creates and configures a new S3 provider instance.
func New(cfg Config, logger log.Logger) (storage.Provider, error) {
	// FIX: Removed deprecated EndpointResolverWithOptions logic.
	// We now configure BaseEndpoint inside s3.NewFromConfig.

	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for S3 provider: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		// FIX: Use BaseEndpoint for custom endpoints (MinIO, LocalStack, etc)
		if cfg.Endpoint != nil && *cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(*cfg.Endpoint)
			o.UsePathStyle = true // Necessary for MinIO
		}
	})

	return &Provider{
		client:        client,
		presignClient: s3.NewPresignClient(client),
		uploader:      manager.NewUploader(client),
		logger:        logger.With(log.String("provider", "s3"), log.String("region", cfg.Region)),
	}, nil
}

func (p *Provider) GeneratePresignedUploadURL(ctx context.Context, bucket, path, mimeType string, size int64, expiresIn time.Duration) (*storage.SignedURL, error) {
	req, err := p.presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(path),
		ContentType: aws.String(mimeType),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiresIn
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate S3 presigned PUT URL: %w", err)
	}
	return &storage.SignedURL{
		URL:       req.URL,
		Method:    "PUT",
		ExpiresAt: time.Now().UTC().Add(expiresIn),
	}, nil
}

func (p *Provider) GeneratePresignedDownloadURL(ctx context.Context, bucket, path string, expiresIn time.Duration) (*storage.SignedURL, error) {
	req, err := p.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(path),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiresIn
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate S3 presigned GET URL: %w", err)
	}
	return &storage.SignedURL{
		URL:       req.URL,
		Method:    "GET",
		ExpiresAt: time.Now().UTC().Add(expiresIn),
	}, nil
}

func (p *Provider) Upload(ctx context.Context, bucket, path string, reader io.Reader, size int64, mimeType string) error {
	_, err := p.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(path),
		Body:        reader,
		ContentType: aws.String(mimeType),
	})
	return err
}

func (p *Provider) Download(ctx context.Context, bucket, path string) (io.ReadCloser, error) {
	output, err := p.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, storage.ErrFileNotFound
		}
		return nil, err
	}
	return output.Body, nil
}

func (p *Provider) Delete(ctx context.Context, bucket, path string) error {
	_, err := p.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(path),
	})
	// S3 does not error if the key doesn't exist, so this is naturally idempotent.
	return err
}

func (p *Provider) Exists(ctx context.Context, bucket, path string) (bool, error) {
	_, err := p.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		var nfe *types.NotFound
		if errors.As(err, &nfe) {
			return false, nil // Correctly handle 404 Not Found as "does not exist".
		}
		return false, err // Another AWS error occurred.
	}
	return true, nil // No error means the object exists.
}
