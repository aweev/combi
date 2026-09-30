package payments

import (
	"context"
	"fmt"

	"github.com/combi/pkg/domain/financial"
	"github.com/combi/pkg/domain/reporting"
)

var _ reporting.DataFetcher = (*fetcher)(nil)

type fetcher struct {
	adminService financial.AdminService
}

func NewReportingFetcher(adminService financial.AdminService) reporting.DataFetcher {
	return &fetcher{
		adminService: adminService,
	}
}

func (f *fetcher) FetchData(ctx context.Context, params map[string]any) (any, error) {
	// 1. Determine Fetch Type
	// We use the "report_type" param if provided, otherwise default to Summary
	reportType := "summary"
	if t, ok := params["report_type"].(string); ok {
		reportType = t
	}

	switch reportType {
	case "summary":
		return f.fetchFinancialSummary(ctx, params)
	case "payouts":
		return f.fetchPayoutReport(ctx, params)
	case "donations":
		return f.fetchDonationLedger(ctx, params)
	default:
		return nil, fmt.Errorf("unsupported financial report type: %s", reportType)
	}
}

func (f *fetcher) fetchFinancialSummary(ctx context.Context, params map[string]any) (any, error) {
	// Pull filter params
	limit := 1000
	offset := 0

	txs, total, err := f.adminService.ListTransactions(ctx, financial.TransactionFilter{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, err
	}

	// Calculate Aggregate Data for the Data Analyst
	var totalVolume int64
	var completedCount int
	var pendingCount int

	statusBreakdown := make(map[string]int)
	typeBreakdown := make(map[string]int64)

	for _, tx := range txs {
		totalVolume += tx.Amount
		statusBreakdown[string(tx.Status)]++
		typeBreakdown[string(tx.Type)] += tx.Amount

		if tx.Status == financial.StatusSucceeded {
			completedCount++
		} else if tx.Status == financial.StatusPending {
			pendingCount++
		}
	}

	return map[string]any{
		"meta": map[string]any{
			"total_transactions": total,
			"report_volume":      totalVolume,
			"currency":           "EUR",
		},
		"summary": map[string]any{
			"completed": completedCount,
			"pending":   pendingCount,
			"failed":    statusBreakdown[string(financial.StatusFailed)],
		},
		"breakdown": typeBreakdown,
		"data":      txs, // Raw data for PDF generation
	}, nil
}

func (f *fetcher) fetchPayoutReport(ctx context.Context, params map[string]any) (any, error) {
	// Similar logic for specific payout reports
	return nil, fmt.Errorf("payout report not yet implemented in fetcher")
}

func (f *fetcher) fetchDonationLedger(ctx context.Context, params map[string]any) (any, error) {
	// Filter for type=DONATION
	st := financial.StatusSucceeded
	txs, total, err := f.adminService.ListTransactions(ctx, financial.TransactionFilter{
		Status: &st,
		Limit:  5000,
	})
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"total": total,
		"items": txs,
	}, nil
}
