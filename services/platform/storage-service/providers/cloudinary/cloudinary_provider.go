package cloudinary

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/log"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api"
	"github.com/cloudinary/cloudinary-go/v2/api/admin"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

// Config holds the configuration for the Cloudinary provider.
// Fields are mapped from configuration (eg. Viper / mapstructure).
type Config struct {
	CloudName    string        `mapstructure:"cloud_name"`
	APIKey       string        `mapstructure:"api_key"`
	APISecret    string        `mapstructure:"api_secret"`
	Timeout      time.Duration `mapstructure:"timeout"`       // per-request timeout (eg. 30s)
	RetryCount   int           `mapstructure:"retry_count"`   // number of retries for transient errors
	RetryBackoff time.Duration `mapstructure:"retry_backoff"` // base backoff duration
	// If true, provider will treat uploaded assets as private and attempt to generate
	// signed download URLs. If false, public URLs (secure_url) are returned.
	UsePrivateURLs bool `mapstructure:"use_private_urls"`
}

// Provider implements the storage.Provider interface using Cloudinary.
type Provider struct {
	cld    *cloudinary.Cloudinary
	cfg    Config
	logger log.Logger
	client *http.Client // optional HTTP client for direct downloads
}

var _ storage.Provider = (*Provider)(nil)

// New constructs a Cloudinary provider with validation and sane defaults.
func New(cfg Config, logger log.Logger) (storage.Provider, error) {
	if cfg.CloudName == "" || cfg.APIKey == "" || cfg.APISecret == "" {
		return nil, errors.New("cloudinary provider requires cloud_name, api_key, and api_secret")
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.RetryCount <= 0 {
		cfg.RetryCount = 3
	}
	if cfg.RetryBackoff == 0 {
		cfg.RetryBackoff = 1 * time.Second
	}

	cld, err := cloudinary.NewFromParams(cfg.CloudName, cfg.APIKey, cfg.APISecret)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cloudinary client: %w", err)
	}

	p := &Provider{
		cld:    cld,
		cfg:    cfg,
		logger: logger.With(log.String("provider", "cloudinary")),
		client: &http.Client{},
	}

	return p, nil
}

func (p *Provider) GeneratePresignedUploadURL(ctx context.Context, bucket, path string, mimeType string, size int64, expiresIn time.Duration) (*storage.SignedURL, error) {
	timestamp := time.Now().Unix()

	// Cloudinary signature parameters
	// PublicID and Timestamp are required for the signature.
	// Eager, transformations, etc. must also be signed if present.
	params := map[string]interface{}{
		"public_id": path,
		"timestamp": timestamp,
		// "folder": bucket, // Optional if we map bucket to folder
	}

	// Generate Signature
	// The SDK has a helper: api.SignParameters(params, apiSecret)
	// We need to import "github.com/cloudinary/cloudinary-go/v2/api"
	signatureParams := url.Values{}
	for key, value := range params {
		signatureParams.Set(key, fmt.Sprint(value))
	}
	signature, err := api.SignParameters(signatureParams, p.cfg.APISecret)
	if err != nil {
		return nil, fmt.Errorf("failed to sign cloudinary params: %w", err)
	}

	// Construct the "URL"
	// We return the standard Cloudinary upload endpoint.
	// The client must send a POST with form-data including file, timestamp, public_id, api_key, signature.
	// To fit into our SignedURL contract, we encode these as query params.
	// Frontend MUST detect this is a Cloudinary URL and handle accordingly,
	// OR use the generic POST /upload/server endpoint if they can't handle this.

	// Standard Cloudinary Upload URL
	baseURL := fmt.Sprintf("https://api.cloudinary.com/v1_1/%s/auto/upload", p.cfg.CloudName)

	// We append the signed params to the URL so the client has them.
	// The client must parse these query params and put them into the POST body.
	u, _ := url.Parse(baseURL)
	q := u.Query()
	q.Set("public_id", path)
	q.Set("timestamp", fmt.Sprintf("%d", timestamp))
	q.Set("api_key", p.cfg.APIKey)
	q.Set("signature", signature)
	u.RawQuery = q.Encode()

	return &storage.SignedURL{
		URL:       u.String(),
		Method:    "POST",                          // Cloudinary uses POST
		ExpiresAt: time.Now().UTC().Add(expiresIn), // Logic handled by signature timestamp validity window (usually valid for 1 hr)
	}, nil
}

// GeneratePresignedDownloadURL returns a time-limited URL for downloading the asset.
// For public assets, Cloudinary exposes the secure URL in the Admin Asset response.
// For private assets, SDKs usually support generating signed private download URLs. If an SDK helper
// is available in your dependency version, replace the implementation below with the SDK call
// (eg. cld.Upload.GeneratePrivateDownloadURL or equivalent). Here we attempt to fetch the asset
// and return either its secure URL (public) or a best-effort signed URL when UsePrivateURLs=true.
func (p *Provider) GeneratePresignedDownloadURL(ctx context.Context, bucket, path string, expiresIn time.Duration) (*storage.SignedURL, error) {
	// Cloudinary treats `path` as the public_id.
	ctx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()

	asset, err := p.cld.Admin.Asset(ctx, admin.AssetParams{PublicID: path})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("asset not found: %w", storage.ErrFileNotFound)
		}
		return nil, fmt.Errorf("failed to get asset metadata: %w", err)
	}

	// If not using private URLs, return the secure URL directly (note: it may not expire).
	if !p.cfg.UsePrivateURLs {
		if asset.SecureURL == "" {
			return nil, fmt.Errorf("asset has no secure url available")
		}
		return &storage.SignedURL{
			URL:       asset.SecureURL,
			Method:    http.MethodGet,
			ExpiresAt: time.Now().UTC().Add(expiresIn),
		}, nil
	}

	// When private URLs are required, many Cloudinary SDKs provide a method to generate
	// a time-limited download link. If such a helper exists in the version you use,
	// it should be used here. As a fallback, you can construct a signed URL using
	// the signing algorithm and your API secret; implementing that here is intentionally
	// omitted to avoid duplicating algorithm code and to keep secrets safe.
	// TODO: Replace the following block with the SDK's GeneratePrivateDownloadURL if available.
	// For now, return the secure URL with a warning (still works if your cloud uses authenticated delivery).
	if asset.SecureURL == "" {
		return nil, fmt.Errorf("asset has no secure url available")
	}

	p.logger.Warn("GeneratePresignedDownloadURL: UsePrivateURLs=true but SDK helper not used; returning secure URL without signature")

	return &storage.SignedURL{
		URL:       asset.SecureURL,
		Method:    http.MethodGet,
		ExpiresAt: time.Now().UTC().Add(expiresIn),
	}, nil
}

// Upload uploads the content read from reader to Cloudinary using PublicID = path.
// bucket is used as an optional namespace prefix to help multi-tenant setups.
func (p *Provider) Upload(ctx context.Context, bucket, path string, reader io.Reader, size int64, mimeType string) error {
	publicID := path
	if bucket != "" {
		publicID = fmt.Sprintf("%s/%s", strings.Trim(bucket, "/"), strings.Trim(path, "/"))
	}

	// per-request timeout
	ctx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()

	var lastErr error
	for attempt := 0; attempt < p.cfg.RetryCount; attempt++ {
		resp, err := p.cld.Upload.Upload(ctx, reader, uploader.UploadParams{
			PublicID:     publicID,
			ResourceType: "auto",
			// You can set additional params here: Folder, Tags, Context, Eager, etc.
		})
		if err == nil {
			// Log metadata for observability
			p.logger.Info("upload successful",
				log.String("public_id", resp.PublicID),
				log.String("secure_url", resp.SecureURL),
			)
			return nil
		}

		lastErr = err
		if !isTransientError(err) {
			// Non-transient error: return immediately
			p.logger.Error("cloudinary upload failed, non-transient", err)
			return fmt.Errorf("cloudinary upload failed: %w", err)
		}

		// Transient: wait and retry
		sleep := time.Duration(attempt+1) * p.cfg.RetryBackoff
		p.logger.Warn("cloudinary upload transient error, retrying", log.Int("attempt", attempt+1), log.Any("backoff", sleep), log.ErrorAttr(err))
		select {
		case <-time.After(sleep):
			// continue
		case <-ctx.Done():
			return fmt.Errorf("upload canceled: %w", ctx.Err())
		}
	}

	p.logger.Warn("cloudinary upload failed after retries")
	return fmt.Errorf("cloudinary upload failed after retries: %w", lastErr)
}

// Download streams the asset from Cloudinary. It obtains a (signed) URL and performs an HTTP GET.
func (p *Provider) Download(ctx context.Context, bucket, path string) (io.ReadCloser, error) {
	// Get a short-lived URL
	signed, err := p.GeneratePresignedDownloadURL(ctx, bucket, path, 5*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("failed to get download url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, signed.URL, nil)
	if err != nil {
		return nil, err
	}

	// Honor provider timeout for the HTTP request.
	// We handle context cancellation by wrapping the response body.
	// This ensures the timeout applies to the request round-trip,
	// and the context is held open while reading the body, then released on Close().
	ctxWithTimeout, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	req = req.WithContext(ctxWithTimeout)

	resp, err := p.client.Do(req)
	if err != nil {
		cancel()
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		// Drain and close
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		cancel()
		return nil, fmt.Errorf("cloudinary download returned non-200 status: %d", resp.StatusCode)
	}

	return &contextCloser{
		ReadCloser: resp.Body,
		cancel:     cancel,
	}, nil
}

// contextCloser wraps an io.ReadCloser and cancels the context on Close.
type contextCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *contextCloser) Close() error {
	defer c.cancel()
	return c.ReadCloser.Close()
}

// Delete removes the asset. Idempotent: treat "not found" as success.
func (p *Provider) Delete(ctx context.Context, bucket, path string) error {
	publicID := path
	if bucket != "" {
		publicID = fmt.Sprintf("%s/%s", strings.Trim(bucket, "/"), strings.Trim(path, "/"))
	}

	ctx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()

	_, err := p.cld.Admin.DeleteAssets(ctx, admin.DeleteAssetsParams{PublicIDs: []string{publicID}})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil
		}
		return fmt.Errorf("cloudinary delete failed: %w", err)
	}
	return nil
}

// Exists checks whether the asset exists.
func (p *Provider) Exists(ctx context.Context, bucket, path string) (bool, error) {
	publicID := path
	if bucket != "" {
		publicID = fmt.Sprintf("%s/%s", strings.Trim(bucket, "/"), strings.Trim(path, "/"))
	}

	ctx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()

	_, err := p.cld.Admin.Asset(ctx, admin.AssetParams{PublicID: publicID})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// HealthCheck performs a lightweight call to validate connectivity and credentials.
// We call Admin.Asset on a deliberately non-existent ID and interpret the response.
func (p *Provider) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()

	// Use a stable-but-unlikely public id to avoid hitting user assets. We only care if the
	// API is reachable and credentials accepted (which typically returns 404 or auth error).
	_, err := p.cld.Admin.Asset(ctx, admin.AssetParams{PublicID: "__health_check__"})
	if err != nil {
		// If the error indicates auth failure, surface it specifically.
		if strings.Contains(strings.ToLower(err.Error()), "unauthorized") || strings.Contains(strings.ToLower(err.Error()), "invalid") {
			return fmt.Errorf("cloudinary health check: authentication failed: %w", err)
		}
		// 404/not found indicates service reachable and credentials valid; treat as healthy.
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			return nil
		}
		// other errors are considered unhealthy
		return fmt.Errorf("cloudinary health check failed: %w", err)
	}
	return nil
}

// isTransientError performs a conservative check for transient errors to decide retry behavior.
func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	l := strings.ToLower(err.Error())
	// common transient indicators
	if strings.Contains(l, "too many requests") || strings.Contains(l, "429") || strings.Contains(l, "timeout") || strings.Contains(l, "502") || strings.Contains(l, "503") || strings.Contains(l, "500") {
		return true
	}
	return false
}
