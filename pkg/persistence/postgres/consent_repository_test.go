// internal/persistence/postgres/consent_repository_test.go
package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/combi/pkg/domain/consent"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsentRepository(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewConsentRepository(db, &log.NoOpLogger{})
	ctx := context.Background()
	userID := uuid.New()

	t.Run("Full_Lifecycle", func(t *testing.T) {
		// 1. Create Agreement (TOS)
		tos := &consent.Agreement{
			ID:          uuid.New(),
			Slug:        "tos",
			Type:        consent.TypeTermsOfService,
			Title:       "Terms of Service",
			IsMandatory: true,
		}
		require.NoError(t, repo.CreateAgreement(ctx, tos))

		// 2. Create Revision v1
		v1 := &consent.Revision{
			ID:          uuid.New(),
			AgreementID: tos.ID,
			Version:     "1.0",
			ContentURL:  "s3://tos-v1",
			ContentHash: "hash1",
			IsActive:    true,
		}
		require.NoError(t, repo.CreateRevision(ctx, v1))

		// 3. Verify Compliance (Should be Missing)
		missing, err := repo.GetMissingMandatoryConsents(ctx, userID)
		require.NoError(t, err)
		assert.Len(t, missing, 1)
		assert.Equal(t, "tos", missing[0].Slug)

		// 4. Grant Consent to v1
		uc := &consent.UserConsent{
			ID:         uuid.New(),
			UserID:     userID,
			RevisionID: v1.ID,
			Status:     consent.StatusActive,
			IPAddress:  "127.0.0.1",
		}
		require.NoError(t, repo.CreateUserConsent(ctx, uc))

		// 5. Verify Compliance (Should be Compliant)
		missingAfter, err := repo.GetMissingMandatoryConsents(ctx, userID)
		require.NoError(t, err)
		assert.Len(t, missingAfter, 0)

		hasConsent, err := repo.HasActiveConsent(ctx, userID, "tos")
		require.NoError(t, err)
		assert.True(t, hasConsent)

		// 6. Publish v2 (New Mandatory Update)
		v2 := &consent.Revision{
			ID:          uuid.New(),
			AgreementID: tos.ID,
			Version:     "2.0",
			ContentURL:  "s3://tos-v2",
			ContentHash: "hash2",
			IsActive:    true,
			PublishedAt: time.Now().Add(time.Second), // Ensure it's newer
		}
		require.NoError(t, repo.CreateRevision(ctx, v2))

		// 7. Verify Compliance (Should be Missing again because v2 is newer)
		missingV2, err := repo.GetMissingMandatoryConsents(ctx, userID)
		require.NoError(t, err)
		assert.Len(t, missingV2, 1)
		assert.Equal(t, "tos", missingV2[0].Slug)
	})

	t.Run("Revocation", func(t *testing.T) {
		// Setup Marketing Agreement (Optional)
		marketing := &consent.Agreement{
			ID:          uuid.New(),
			Slug:        "newsletter",
			Type:        consent.TypeMarketing,
			IsMandatory: false,
		}
		require.NoError(t, repo.CreateAgreement(ctx, marketing))

		rev := &consent.Revision{ID: uuid.New(), AgreementID: marketing.ID, Version: "1", ContentURL: "x", ContentHash: "x", IsActive: true}
		require.NoError(t, repo.CreateRevision(ctx, rev))

		uc := &consent.UserConsent{
			ID:         uuid.New(),
			UserID:     userID,
			RevisionID: rev.ID,
			Status:     consent.StatusActive,
		}
		require.NoError(t, repo.CreateUserConsent(ctx, uc))

		// Revoke
		require.NoError(t, repo.RevokeUserConsent(ctx, userID, marketing.ID, time.Now()))

		// Check status
		stored, err := repo.GetUserConsent(ctx, userID, rev.ID)
		require.NoError(t, err)
		assert.Equal(t, consent.StatusRevoked, stored.Status)
		assert.NotNil(t, stored.RevokedAt)

		// Check boolean check
		has, err := repo.HasActiveConsent(ctx, userID, "newsletter")
		require.NoError(t, err)
		assert.False(t, has)
	})
}
