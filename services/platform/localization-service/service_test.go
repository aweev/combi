package localization_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	audit_mocks "github.com/combi/pkg/domain/audit/mocks"
	"github.com/combi/pkg/domain/auth"
	"github.com/combi/pkg/domain/localization"
	loc_mocks "github.com/combi/pkg/domain/localization/mocks"
	cache_mocks "github.com/combi/pkg/cache/mocks"
	"github.com/combi/pkg/log"
	loc_service "github.com/combi/localization"
)

func TestService_GetBundle(t *testing.T) {
	repo := loc_mocks.NewMockRepository(t)
	cacheSvc := cache_mocks.NewMockService(t)
	svc := loc_service.NewService(repo, cacheSvc, nil, nil, &log.NoOpLogger{})
	ctx := context.Background()

	repo.EXPECT().GetValuesForNamespace(ctx, localization.LocaleEnUS, "common").
		Return(map[string]string{"btn_save": "Save", "btn_cancel": "Cancel"}, nil).Once()

	repo.EXPECT().GetValuesForNamespace(ctx, localization.LocaleEnUS, "auth").
		Return(map[string]string{"login_title": "Sign In"}, nil).Once()

	bundle, err := svc.GetBundle(ctx, localization.LocaleEnUS, []string{"common", "auth"})
	require.NoError(t, err)

	// Verify merging logic (namespace.key)
	assert.Equal(t, "Save", bundle["common.btn_save"])
	assert.Equal(t, "Sign In", bundle["auth.login_title"])
}

func TestService_CreateKey(t *testing.T) {
	repo := loc_mocks.NewMockRepository(t)
	auditRec := audit_mocks.NewMockEventRecorder(t)
	svc := loc_service.NewService(repo, nil, nil, auditRec, &log.NoOpLogger{})

	actorID := uuid.New()
	ctx := auth.NewContextWithActor(context.Background(), &auth.Actor{AccountID: actorID})

	key := localization.TranslationKey{Namespace: "home", Key: "hero_title"}

	repo.EXPECT().CreateKey(ctx, mock.Anything).Return(nil).Once()
	auditRec.EXPECT().Record(ctx, mock.Anything).Return(nil).Once()

	created, err := svc.CreateKey(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, "hero_title", created.Key)
}
