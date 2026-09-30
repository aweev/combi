package loader_test

import (
	"context"
	"errors"
	"testing"

	"github.com/combi/pkg/compliance/kyctiers"
	"github.com/combi/pkg/compliance/limits"
	"github.com/combi/pkg/compliance/loader"
	"github.com/shopspring/decimal"
)

// mockRemote simulates the svc-compliance gRPC connection.
type mockRemote struct {
	calls         int
	shouldFail    bool
	kycResponse   kyctiers.Tier
	limitResponse limits.Limit
}

func (m *mockRemote) FetchKYCTier(ctx context.Context, country string, level int) (kyctiers.Tier, error) {
	m.calls++
	if m.shouldFail {
		return kyctiers.Tier{}, errors.New("svc-compliance unreachable")
	}
	return m.kycResponse, nil
}

func (m *mockRemote) FetchLimit(ctx context.Context, country string, level int) (limits.Limit, error) {
	m.calls++
	if m.shouldFail {
		return limits.Limit{}, errors.New("svc-compliance unreachable")
	}
	return m.limitResponse, nil
}

func TestLoader_CacheMiss_CallsComplianceService(t *testing.T) {
	mock := &mockRemote{
		limitResponse: limits.Limit{
			Country: "CM", KYCLevel: 1, DailySendUSD: decimal.NewFromInt(999), Notes: "Live Override",
		},
	}
	l := loader.New(mock)

	// First call misses cache, hits remote
	limit := l.Limit(context.Background(), "CM", 1)

	if mock.calls != 1 {
		t.Errorf("Expected 1 call to remote, got %d", mock.calls)
	}
	if !limit.DailySendUSD.Equal(decimal.NewFromInt(999)) {
		t.Errorf("Expected live limit 999, got %s", limit.DailySendUSD.String())
	}
}

func TestLoader_CacheHit_NoGRPCCall(t *testing.T) {
	mock := &mockRemote{
		limitResponse: limits.Limit{
			Country: "CM", KYCLevel: 1, DailySendUSD: decimal.NewFromInt(999),
		},
	}
	l := loader.New(mock)

	// Call 1 (Miss -> Fetch)
	l.Limit(context.Background(), "CM", 1)
	// Call 2 (Hit -> Cache)
	l.Limit(context.Background(), "CM", 1)

	if mock.calls != 1 {
		t.Errorf("Expected exactly 1 network call due to caching, got %d", mock.calls)
	}
}

func TestLoader_ComplianceServiceDown_UsesEmbeddedDefault(t *testing.T) {
	mock := &mockRemote{shouldFail: true} // Simulate gRPC dial failure
	l := loader.New(mock)

	// Remote fails, so it should fall back to the limits.yaml embedded data
	limit := l.Limit(context.Background(), "CM", 1)

	if !limit.DailySendUSD.Equal(decimal.NewFromInt(200)) {
		t.Errorf("Expected fallback to embedded 200 USD limit, got %s", limit.DailySendUSD.String())
	}
}

func TestLoader_InvalidateCache_RefetchesOnNextCall(t *testing.T) {
	mock := &mockRemote{
		limitResponse: limits.Limit{
			Country: "CM", KYCLevel: 1, DailySendUSD: decimal.NewFromInt(999),
		},
	}
	l := loader.New(mock)

	// Call 1
	l.Limit(context.Background(), "CM", 1)

	// Invalidate cache (simulating event bus compliance.rules_updated event)
	l.InvalidateCache("limit:CM:1")

	// Call 2
	l.Limit(context.Background(), "CM", 1)

	if mock.calls != 2 {
		t.Errorf("Expected 2 network calls after cache invalidation, got %d", mock.calls)
	}
}
