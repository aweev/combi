package postgres_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/combi/pkg/domain/reporting"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportingRepository(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewReportingRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	t.Run("Templates CRUD", func(t *testing.T) {
		slug := "annual-impact-2024"
		tmpl := &reporting.ReportTemplate{
			ID:          uuid.New(),
			Slug:        slug,
			Name:        "Annual Report",
			Type:        reporting.TypeImpactReport,
			Content:     "<h1>Report</h1>",
			ParamSchema: json.RawMessage(`{"type":"object"}`),
		}

		require.NoError(t, repo.CreateTemplate(ctx, tmpl))

		fetched, err := repo.GetTemplateBySlug(ctx, slug)
		require.NoError(t, err)
		assert.Equal(t, tmpl.ID, fetched.ID)
		assert.JSONEq(t, string(tmpl.ParamSchema), string(fetched.ParamSchema))

		list, err := repo.ListTemplates(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, list)
	})

	t.Run("Report Lifecycle", func(t *testing.T) {
		userID := uuid.New()
		tmplID := uuid.New() // Just a mock ID, foreign key constraints might need setup if strict, but let's assume we use the template from above

		// Create Template first to satisfy FK if enforced by DB (it is)
		tmpl := &reporting.ReportTemplate{
			ID:          tmplID,
			Slug:        "financial-summary",
			Name:        "Finances",
			Type:        reporting.TypeFinancialSummary,
			Content:     "...",
			ParamSchema: json.RawMessage(`{}`),
		}
		require.NoError(t, repo.CreateTemplate(ctx, tmpl))

		// 1. Create Report Request
		report := &reporting.GeneratedReport{
			ID:          uuid.New(),
			TemplateID:  &tmplID,
			RequesterID: userID,
			Type:        reporting.TypeFinancialSummary,
			Status:      reporting.StatusPending,
			Format:      reporting.FormatPDF,
			Parameters:  json.RawMessage(`{"year": 2024}`),
		}
		require.NoError(t, repo.CreateReport(ctx, report))

		// 2. Fetch
		fetched, err := repo.GetReport(ctx, report.ID)
		require.NoError(t, err)
		assert.Equal(t, reporting.StatusPending, fetched.Status)

		// 3. Update Status (Processing -> Completed)
		fileURL := "https://s3.amazonaws.com/reports/123.pdf"
		require.NoError(t, repo.UpdateReportStatus(ctx, report.ID, reporting.StatusCompleted, &fileURL, nil))

		// 4. Verify Completion
		completed, err := repo.GetReport(ctx, report.ID)
		require.NoError(t, err)
		assert.Equal(t, reporting.StatusCompleted, completed.Status)
		assert.NotNil(t, completed.CompletedAt)
		assert.Equal(t, fileURL, *completed.FileURL)

		// 5. List
		list, total, err := repo.ListReports(ctx, userID, 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Len(t, list, 1)
	})
}
