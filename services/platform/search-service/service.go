package search

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/rbac"
	"github.com/combi/pkg/domain/search"
	"github.com/combi/pkg/log"
)

var _ search.Service = (*service)(nil)

type service struct {
	engine      search.Engine
	rbacService rbac.AuthorizationService
	dispatcher  queue.Dispatcher
	logger      log.Logger
	providers   map[search.IndexName]search.DocumentProvider
}

func NewService(
	engine search.Engine,
	rbacSvc rbac.AuthorizationService,
	dispatcher queue.Dispatcher,
	logger log.Logger,
) *service {
	return &service{
		engine:      engine,
		rbacService: rbacSvc,
		dispatcher:  dispatcher,
		logger:      logger.With(log.String("service", "search")),
		providers:   make(map[search.IndexName]search.DocumentProvider),
	}
}

func (s *service) RegisterProvider(index search.IndexName, provider search.DocumentProvider) {
	s.providers[index] = provider
}

func (s *service) Search(ctx context.Context, params search.SearchParams) (*search.SearchResponse, error) {
	// 1. Resolve Viewers Scopes
	// We trust the Handler to populate ViewerID/Roles.
	scopes := []string{"public"}

	if params.ViewerID != uuid.Nil {
		scopes = append(scopes, fmt.Sprintf("user:%s", params.ViewerID.String()))
		for _, role := range params.ViewerRoles {
			scopes = append(scopes, fmt.Sprintf("role:%s", role))
		}
		for _, org := range params.ViewerOrgIDs {
			scopes = append(scopes, fmt.Sprintf("org:%s", org.String()))
		}
	}

	query := search.SearchQuery{
		Index:        params.Index,
		Term:         params.Query,
		Filters:      params.Filters,
		Facets:       params.Facets,
		Limit:        params.Limit,
		Offset:       (params.Page - 1) * params.Limit,
		ViewerScopes: scopes,
	}

	return s.engine.Search(ctx, query)
}

func (s *service) IndexDocument(ctx context.Context, doc search.SearchDocument) error {
	return s.engine.IndexBatch(ctx, []search.SearchDocument{doc})
}

func (s *service) DeleteDocument(ctx context.Context, id string) error {
	return s.engine.DeleteBatch(ctx, []string{id})
}

func (s *service) TriggerRebuild(ctx context.Context, index search.IndexName) error {
	if _, ok := s.providers[index]; !ok {
		return fmt.Errorf("no provider registered for index: %s", index)
	}

	payload, _ := json.Marshal(map[string]string{"index": string(index)})
	_, err := s.dispatcher.Dispatch(ctx, search.JobRebuildIndex, payload, queue.WithPriority(10))
	return err
}

// ProcessRebuildJob orchestrates the Blue/Green rebuild
func (s *service) ProcessRebuildJob(ctx context.Context, indexName search.IndexName) error {
	s.logger.Info("starting index rebuild", log.String("index", string(indexName)))

	// 1. Prepare Staging
	if err := s.engine.PrepareRebuild(ctx); err != nil {
		return fmt.Errorf("failed to prepare staging table: %w", err)
	}

	provider, exists := s.providers[indexName]
	if !exists {
		return fmt.Errorf("no provider registered for index: %s", indexName)
	}

	// 2. Stream Data to Staging
	docChan := make(chan search.SearchDocument, 100)
	errChan := make(chan error, 1)

	// Producer
	go func() {
		defer close(docChan)
		if err := provider.IterateDocs(ctx, docChan); err != nil {
			errChan <- err
		}
	}()

	// Consumer (Batcher)
	batch := make([]search.SearchDocument, 0, 500) // Batch size 500
	for doc := range docChan {
		batch = append(batch, doc)
		if len(batch) >= 500 {
			if err := s.engine.IndexBatchStaging(ctx, batch); err != nil {
				return fmt.Errorf("staging indexing failed: %w", err)
			}
			batch = batch[:0]
		}
	}

	// Flush remaining
	if len(batch) > 0 {
		if err := s.engine.IndexBatchStaging(ctx, batch); err != nil {
			return fmt.Errorf("staging indexing failed: %w", err)
		}
	}

	// Check for producer error
	select {
	case err := <-errChan:
		return fmt.Errorf("provider iteration failed: %w", err)
	default:
	}

	// 3. Commit (Swap)
	if err := s.engine.CommitRebuild(ctx); err != nil {
		return fmt.Errorf("failed to swap tables: %w", err)
	}

	s.logger.Info("index rebuild complete", log.String("index", string(indexName)))
	return nil
}
