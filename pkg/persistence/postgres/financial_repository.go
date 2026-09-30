package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/financial"
	"github.com/combi/pkg/domain/organization"
	"github.com/combi/pkg/log"
	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var _ financial.Repository = (*FinancialRepository)(nil)

type FinancialRepository struct {
	db     *sqlx.DB
	runner sqlx.ExtContext
	sq     squirrel.StatementBuilderType
	logger log.Logger
}

func NewFinancialRepository(db *sqlx.DB, logger log.Logger) *FinancialRepository {
	return &FinancialRepository{
		db:     db,
		runner: db,
		sq:     squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
		logger: logger.With(log.String("component", "financial_repo")),
	}
}

// WithTx implements the transactional boundary.
func (r *FinancialRepository) WithTx(ctx context.Context, fn func(txRepo financial.Repository) error) (err error) {
	if _, ok := r.runner.(*sqlx.Tx); ok {
		return errors.New("nested transactions not supported")
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	return fn(&FinancialRepository{
		db:     r.db,
		runner: tx,
		sq:     r.sq,
		logger: r.logger,
	})
}

// --- Transactions ---

func (r *FinancialRepository) CreateTransaction(ctx context.Context, tx *financial.Transaction) error {
	metaBytes, _ := json.Marshal(tx.Metadata)

	var orgID *uuid.UUID
	if org := organization.OrgFromContext(ctx); org != nil {
		orgID = &org.OrgID
	}

	query, args, _ := r.sq.Insert("transactions").
		Columns(
			"id", "organization_id", "user_id", "type", "status", "amount", "currency", "description",
			"provider", "provider_id", "related_transaction_id", "processed_at",
			"reference_type", "reference_id", "metadata",
		).
		Values(
			tx.ID, orgID, tx.UserID, tx.Type, tx.Status, tx.Amount, tx.Currency, tx.Description,
			tx.Provider, tx.ProviderID, tx.RelatedTransactionID, tx.ProcessedAt,
			tx.ReferenceType, tx.ReferenceID, metaBytes,
		).
		ToSql()

	_, err := r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *FinancialRepository) GetTransaction(ctx context.Context, id uuid.UUID) (*financial.Transaction, error) {
	var dest dbTransaction
	builder := WithTenant(ctx, r.sq.Select("*").From("transactions").Where(squirrel.Eq{"id": id}))
	query, args, _ := builder.ToSql()
	if err := sqlx.GetContext(ctx, r.runner, &dest, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, financial.ErrTransactionNotFound
		}
		return nil, err
	}
	return dest.toDomain(), nil
}

func (r *FinancialRepository) UpdateTransaction(ctx context.Context, tx *financial.Transaction) error {
	query, args, _ := r.sq.Update("transactions").
		Set("status", tx.Status).
		Set("provider_id", tx.ProviderID).
		Set("processed_at", tx.ProcessedAt).
		Where(squirrel.Eq{"id": tx.ID}).
		ToSql()
	_, err := r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *FinancialRepository) ListTransactions(ctx context.Context, filter financial.TransactionFilter) ([]*financial.Transaction, int, error) {
	builder := WithTenant(ctx, r.sq.Select("*, COUNT(*) OVER() as total_count").From("transactions"))

	if filter.UserID != nil {
		builder = builder.Where(squirrel.Eq{"user_id": *filter.UserID})
	}
	if filter.Type != nil {
		builder = builder.Where(squirrel.Eq{"type": *filter.Type})
	}
	if filter.Status != nil {
		builder = builder.Where(squirrel.Eq{"status": *filter.Status})
	}

	// Pagination
	limit := uint64(20)
	if filter.Limit > 0 {
		limit = uint64(filter.Limit)
	}
	offset := uint64(0)
	if filter.Offset > 0 {
		offset = uint64(filter.Offset)
	}

	query, args, err := builder.
		OrderBy("processed_at DESC").
		Limit(limit).
		Offset(offset).
		ToSql()
	if err != nil {
		return nil, 0, err
	}

	// Use a struct that includes Total for the window function result
	type dbTransactionWithTotal struct {
		dbTransaction
		Total int `db:"total_count"`
	}
	var rows []dbTransactionWithTotal

	if err := sqlx.SelectContext(ctx, r.runner, &rows, query, args...); err != nil {
		return nil, 0, err
	}

	if len(rows) == 0 {
		return []*financial.Transaction{}, 0, nil
	}

	transactions := make([]*financial.Transaction, len(rows))
	for i, row := range rows {
		transactions[i] = row.toDomain()
	}

	return transactions, rows[0].Total, nil
}

// --- Subscriptions ---

func (r *FinancialRepository) CreateSubscription(ctx context.Context, sub *financial.Subscription) error {
	query, args, _ := r.sq.Insert("subscriptions").
		Columns("id", "user_id", "price_id", "status", "provider", "provider_id", "current_period_start", "current_period_end").
		Values(sub.ID, sub.UserID, sub.PriceID, sub.Status, sub.Provider, sub.ProviderID, sub.CurrentPeriodStart, sub.CurrentPeriodEnd).
		ToSql()
	_, err := r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *FinancialRepository) GetSubscription(ctx context.Context, id uuid.UUID) (*financial.Subscription, error) {
	var dest dbSubscription
	builder := WithTenant(ctx, r.sq.Select("*").From("subscriptions").Where(squirrel.Eq{"id": id}))
	query, args, _ := builder.ToSql()
	if err := sqlx.GetContext(ctx, r.runner, &dest, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, financial.ErrSubscriptionNotFound
		}
		return nil, err
	}
	return dest.toDomain(), nil
}

func (r *FinancialRepository) UpdateSubscription(ctx context.Context, sub *financial.Subscription) error {
	query, args, _ := r.sq.Update("subscriptions").
		Set("status", sub.Status).
		Set("current_period_start", sub.CurrentPeriodStart).
		Set("current_period_end", sub.CurrentPeriodEnd).
		Set("cancelled_at", sub.CancelledAt).
		Set("ended_at", sub.EndedAt).
		Where(squirrel.Eq{"id": sub.ID}).
		ToSql()
	_, err := r.runner.ExecContext(ctx, query, args...)
	return err
}

func (r *FinancialRepository) ListSubscriptionsForUser(ctx context.Context, userID uuid.UUID) ([]*financial.Subscription, error) {
	var dest []dbSubscription
	builder := WithTenant(ctx, r.sq.Select("*").From("subscriptions").Where(squirrel.Eq{"user_id": userID}))
	query, args, _ := builder.ToSql()
	if err := sqlx.SelectContext(ctx, r.runner, &dest, query, args...); err != nil {
		return nil, err
	}
	list := make([]*financial.Subscription, len(dest))
	for i, d := range dest {
		list[i] = d.toDomain()
	}
	return list, nil
}

// --- Products & Prices ---

func (r *FinancialRepository) CreateProduct(ctx context.Context, p *financial.Product) error {
	q1, a1, _ := r.sq.Insert("products").
		Columns("id", "type", "name", "description", "is_active", "created_at").
		Values(p.ID, p.Type, p.Name, p.Description, p.IsActive, p.CreatedAt).ToSql()
	if _, err := r.runner.ExecContext(ctx, q1, a1...); err != nil {
		return err
	}

	for _, price := range p.Prices {
		q2, a2, _ := r.sq.Insert("prices").
			Columns("id", "product_id", "amount", "currency", "interval", "provider_price_id", "is_active").
			Values(price.ID, price.ProductID, price.Amount, price.Currency, price.Interval, price.ProviderPriceID, price.IsActive).
			ToSql()
		if _, err := r.runner.ExecContext(ctx, q2, a2...); err != nil {
			return err
		}
	}
	return nil
}

func (r *FinancialRepository) GetProduct(ctx context.Context, id uuid.UUID) (*financial.Product, error) {
	// Fetch Product
	var dbp dbProduct
	builder := WithTenant(ctx, r.sq.Select("*").From("products").Where(squirrel.Eq{"id": id}))
	query, args, _ := builder.ToSql()
	if err := sqlx.GetContext(ctx, r.runner, &dbp, query, args...); err != nil {
		return nil, err
	}
	// Fetch Prices
	var dbPrices []dbPrice
	pQuery, pArgs, _ := r.sq.Select("*").From("prices").Where(squirrel.Eq{"product_id": id}).ToSql()
	if err := sqlx.SelectContext(ctx, r.runner, &dbPrices, pQuery, pArgs...); err != nil {
		return nil, err
	}

	return dbp.toDomain(dbPrices), nil
}

func (r *FinancialRepository) UpdateProduct(ctx context.Context, p *financial.Product) error {
	q, a, _ := r.sq.Update("products").
		Set("name", p.Name).
		Set("description", p.Description).
		Set("is_active", p.IsActive).
		Where(squirrel.Eq{"id": p.ID}).ToSql()
	_, err := r.runner.ExecContext(ctx, q, a...)
	return err
}

func (r *FinancialRepository) ListProducts(ctx context.Context) ([]*financial.Product, error) {
	// Eager load prices if possible, or fetch separate
	// For simplicity in SQLx, usually we fetch products then prices, or use a JOIN.
	// Here we fetch products first.
	query := `SELECT * FROM products WHERE is_active = true ORDER BY created_at DESC`
	var products []*financial.Product
	if err := r.db.SelectContext(ctx, &products, query); err != nil {
		return nil, fmt.Errorf("failed to list products: %w", err)
	}

	if len(products) == 0 {
		return products, nil
	}

	// Fetch Prices for these products
	prodIDs := make([]uuid.UUID, len(products))
	for i, p := range products {
		prodIDs[i] = p.ID
	}

	// Helper to fetch prices in bulk
	pricesMap, err := r.getPricesForProducts(ctx, prodIDs)
	if err != nil {
		return nil, err
	}

	// Assign prices
	for _, p := range products {
		if prices, ok := pricesMap[p.ID]; ok {
			p.Prices = prices
		} else {
			p.Prices = []financial.Price{}
		}
	}

	return products, nil
}

func (r *FinancialRepository) getPricesForProducts(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID][]financial.Price, error) {
	query, args, err := sqlx.In(`SELECT * FROM prices WHERE product_id IN (?) AND is_active = true`, productIDs)
	if err != nil {
		return nil, err
	}
	query = r.db.Rebind(query)

	var prices []financial.Price
	if err := r.db.SelectContext(ctx, &prices, query, args...); err != nil {
		return nil, fmt.Errorf("failed to fetch prices: %w", err)
	}

	result := make(map[uuid.UUID][]financial.Price)
	for _, p := range prices {
		result[p.ProductID] = append(result[p.ProductID], p)
	}
	return result, nil
}

func (r *FinancialRepository) GetPrice(ctx context.Context, id uuid.UUID) (*financial.Price, error) {
	var dbp dbPrice
	q, a, _ := r.sq.Select("*").From("prices").Where(squirrel.Eq{"id": id}).ToSql()
	if err := sqlx.GetContext(ctx, r.runner, &dbp, q, a...); err != nil {
		return nil, err
	}
	return dbp.toDomain(), nil
}

func (r *FinancialRepository) GetPricesByIDs(ctx context.Context, ids []uuid.UUID) ([]*financial.Price, error) {
	var dbPrices []dbPrice
	q, a, _ := r.sq.Select("*").From("prices").Where(squirrel.Eq{"id": ids}).ToSql()
	if err := sqlx.SelectContext(ctx, r.runner, &dbPrices, q, a...); err != nil {
		return nil, err
	}
	res := make([]*financial.Price, len(dbPrices))
	for i, p := range dbPrices {
		res[i] = p.toDomain()
	}
	return res, nil
}

// --- Payment Methods ---

func (r *FinancialRepository) CreatePaymentMethod(ctx context.Context, pm *financial.PaymentMethod) error {
	details, _ := json.Marshal(pm.Details)
	q, a, _ := r.sq.Insert("payment_methods").
		Columns("id", "user_id", "provider", "provider_id", "type", "details", "is_default", "created_at").
		Values(pm.ID, pm.UserID, pm.Provider, pm.ProviderID, pm.Type, details, pm.IsDefault, pm.CreatedAt).
		ToSql()
	_, err := r.runner.ExecContext(ctx, q, a...)
	return err
}

func (r *FinancialRepository) GetPaymentMethod(ctx context.Context, id uuid.UUID) (*financial.PaymentMethod, error) {
	var dbpm dbPaymentMethod
	q, a, _ := r.sq.Select("*").From("payment_methods").Where(squirrel.Eq{"id": id}).ToSql()
	if err := sqlx.GetContext(ctx, r.runner, &dbpm, q, a...); err != nil {
		return nil, err
	}
	return dbpm.toDomain(), nil
}

func (r *FinancialRepository) UpdatePaymentMethod(ctx context.Context, pm *financial.PaymentMethod) error {
	q, a, _ := r.sq.Update("payment_methods").
		Set("is_default", pm.IsDefault).
		Where(squirrel.Eq{"id": pm.ID}).ToSql()
	_, err := r.runner.ExecContext(ctx, q, a...)
	return err
}

func (r *FinancialRepository) DeletePaymentMethod(ctx context.Context, id uuid.UUID) error {
	q, a, _ := r.sq.Delete("payment_methods").Where(squirrel.Eq{"id": id}).ToSql()
	_, err := r.runner.ExecContext(ctx, q, a...)
	return err
}

func (r *FinancialRepository) ListPaymentMethodsForUser(ctx context.Context, userID uuid.UUID) ([]*financial.PaymentMethod, error) {
	var dbMethods []dbPaymentMethod
	q, a, _ := r.sq.Select("*").From("payment_methods").Where(squirrel.Eq{"user_id": userID}).ToSql()
	if err := sqlx.SelectContext(ctx, r.runner, &dbMethods, q, a...); err != nil {
		return nil, err
	}

	res := make([]*financial.PaymentMethod, len(dbMethods))
	for i, m := range dbMethods {
		res[i] = m.toDomain()
	}
	return res, nil
}

func (r *FinancialRepository) GetDefaultPaymentMethodForUser(ctx context.Context, userID uuid.UUID) (*financial.PaymentMethod, error) {
	var dbpm dbPaymentMethod
	q, a, _ := r.sq.Select("*").From("payment_methods").
		Where(squirrel.Eq{"user_id": userID, "is_default": true}).ToSql()
	if err := sqlx.GetContext(ctx, r.runner, &dbpm, q, a...); err != nil {
		return nil, err
	}
	return dbpm.toDomain(), nil
}

func (r *FinancialRepository) CountPaymentMethodsForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	q, a, _ := r.sq.Select("COUNT(*)").From("payment_methods").Where(squirrel.Eq{"user_id": userID}).ToSql()
	err := sqlx.GetContext(ctx, r.runner, &count, q, a...)
	return count, err
}

func (r *FinancialRepository) ClearDefaultPaymentMethod(ctx context.Context, userID uuid.UUID) error {
	q, a, _ := r.sq.Update("payment_methods").
		Set("is_default", false).
		Where(squirrel.Eq{"user_id": userID}).ToSql()
	_, err := r.runner.ExecContext(ctx, q, a...)
	return err
}

// --- Customers ---

func (r *FinancialRepository) CreateCustomer(ctx context.Context, c *financial.Customer) error {
	q, a, _ := r.sq.Insert("customers").
		Columns("id", "user_id", "provider", "provider_id", "created_at", "updated_at").
		Values(c.ID, c.UserID, c.Provider, c.ProviderID, time.Now().UTC(), time.Now().UTC()).ToSql()
	_, err := r.runner.ExecContext(ctx, q, a...)
	return err
}

func (r *FinancialRepository) GetCustomerByUserID(ctx context.Context, userID uuid.UUID) (*financial.Customer, error) {
	var dbc dbCustomer
	q, a, _ := r.sq.Select("*").From("customers").Where(squirrel.Eq{"user_id": userID}).ToSql()
	if err := sqlx.GetContext(ctx, r.runner, &dbc, q, a...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, financial.ErrCustomerNotFound
		}
		return nil, err
	}
	return dbc.toDomain(), nil
}

// --- Invoices ---

func (r *FinancialRepository) CreateInvoice(ctx context.Context, inv *financial.Invoice) error {
	q, a, _ := r.sq.Insert("invoices").
		Columns("id", "user_id", "subscription_id", "status", "amount_due", "currency", "due_date", "period_start", "period_end", "created_at", "updated_at").
		Values(inv.ID, inv.UserID, inv.SubscriptionID, inv.Status, inv.AmountDue, inv.Currency, inv.DueDate, inv.PeriodStart, inv.PeriodEnd, inv.CreatedAt, inv.UpdatedAt).
		ToSql()
	_, err := r.runner.ExecContext(ctx, q, a...)
	return err
}

func (r *FinancialRepository) GetInvoice(ctx context.Context, id uuid.UUID) (*financial.Invoice, error) {
	var dbi dbInvoice
	q, a, _ := r.sq.Select("*").From("invoices").Where(squirrel.Eq{"id": id}).ToSql()
	if err := sqlx.GetContext(ctx, r.runner, &dbi, q, a...); err != nil {
		return nil, err
	}
	return dbi.toDomain(), nil
}

func (r *FinancialRepository) UpdateInvoice(ctx context.Context, inv *financial.Invoice) error {
	inv.UpdatedAt = time.Now().UTC()
	q, a, _ := r.sq.Update("invoices").
		Set("status", inv.Status).
		Set("transaction_id", inv.TransactionID).
		Set("paid_at", inv.PaidAt).
		Set("updated_at", inv.UpdatedAt).
		Where(squirrel.Eq{"id": inv.ID}).ToSql()
	_, err := r.runner.ExecContext(ctx, q, a...)
	return err
}

func (r *FinancialRepository) ListInvoicesForUser(ctx context.Context, userID uuid.UUID) ([]*financial.Invoice, error) {
	var dbs []dbInvoice
	q, a, _ := r.sq.Select("*").From("invoices").Where(squirrel.Eq{"user_id": userID}).OrderBy("created_at DESC").ToSql()
	if err := sqlx.SelectContext(ctx, r.runner, &dbs, q, a...); err != nil {
		return nil, err
	}
	res := make([]*financial.Invoice, len(dbs))
	for i, d := range dbs {
		res[i] = d.toDomain()
	}
	return res, nil
}

func (r *FinancialRepository) GetDueInvoices(ctx context.Context, dueBefore time.Time, limit int) ([]*financial.Invoice, error) {
	q, a, _ := r.sq.Select("*").From("invoices").
		Where(squirrel.Eq{"status": financial.StatusInvoiceOpen}).
		Where(squirrel.LtOrEq{"due_date": dueBefore}).
		Limit(uint64(limit)).ToSql()

	var dbs []dbInvoice
	if err := sqlx.SelectContext(ctx, r.runner, &dbs, q, a...); err != nil {
		return nil, err
	}
	res := make([]*financial.Invoice, len(dbs))
	for i, d := range dbs {
		res[i] = d.toDomain()
	}
	return res, nil
}

// --- Ledger ---

func (r *FinancialRepository) CreateJournalEntry(ctx context.Context, entry *financial.JournalEntry) error {
	q1, a1, _ := r.sq.Insert("journal_entries").
		Columns("id", "transaction_id", "description", "posted_at").
		Values(entry.ID, entry.TransactionID, entry.Description, entry.PostedAt).ToSql()
	if _, err := r.runner.ExecContext(ctx, q1, a1...); err != nil {
		return err
	}

	for _, le := range entry.Entries {
		q2, a2, _ := r.sq.Insert("ledger_entries").
			Columns("id", "journal_entry_id", "account_id", "direction", "amount").
			Values(le.ID, entry.ID, le.AccountID, le.Direction, le.Amount).ToSql()
		if _, err := r.runner.ExecContext(ctx, q2, a2...); err != nil {
			return err
		}
	}
	return nil
}

func (r *FinancialRepository) GetLedgerAccountByName(ctx context.Context, name string) (*financial.Account, error) {
	var dba dbLedgerAccount
	q, a, _ := r.sq.Select("*").From("ledger_accounts").Where(squirrel.Eq{"name": name}).ToSql()
	if err := sqlx.GetContext(ctx, r.runner, &dba, q, a...); err != nil {
		return nil, err
	}
	return dba.toDomain(), nil
}

func (r *FinancialRepository) ScanDueSubscriptions(ctx context.Context, cutoff time.Time, limit int) ([]*financial.Subscription, error) {
	query, args, _ := r.sq.Select("*").From("subscriptions").
		Where(squirrel.Eq{"status": financial.SubStatusActive}).
		Where(squirrel.LtOrEq{"current_period_end": cutoff}).
		Limit(uint64(limit)).
		ToSql()

	var dest []dbSubscription
	if err := sqlx.SelectContext(ctx, r.runner, &dest, query, args...); err != nil {
		return nil, err
	}

	list := make([]*financial.Subscription, len(dest))
	for i, d := range dest {
		list[i] = d.toDomain()
	}
	return list, nil
}

func (r *FinancialRepository) GetAnnualFinancialSummary(ctx context.Context, year int) (*financial.AnnualSummaryDTO, error) {
	// Define time range for the year
	start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(1, 0, 0)

	// 1. Aggregates (Revenue, Refunds, Count)
	// Assumes 'amount' is positive for payments. Refunds are Type=REFUND.
	// Net = Payment - Refund.

	// Note: In double-entry ledger, we query the ledger.
	// But often for simple summaries, querying the Transaction table is faster/easier if it's accurate.
	// Let's query Transactions for operational reporting.

	statsQuery := `
		SELECT 
			COALESCE(SUM(CASE WHEN type = 'PAYMENT' THEN amount ELSE 0 END), 0) as revenue,
			COALESCE(SUM(CASE WHEN type = 'REFUND' THEN amount ELSE 0 END), 0) as refunds,
			COUNT(*) as count
		FROM transactions 
		WHERE status = 'SUCCEEDED' 
		  AND processed_at >= $1 
		  AND processed_at < $2
	`

	var stats struct {
		Revenue int64 `db:"revenue"`
		Refunds int64 `db:"refunds"`
		Count   int   `db:"count"`
	}

	if err := r.db.GetContext(ctx, &stats, statsQuery, start, end); err != nil {
		return nil, fmt.Errorf("failed to aggregate annual stats: %w", err)
	}

	// 2. Monthly Breakdown
	monthlyQuery := `
		SELECT 
			EXTRACT(MONTH FROM processed_at)::int as month,
			COALESCE(SUM(CASE WHEN type = 'PAYMENT' THEN amount ELSE -amount END), 0) as net_amount
		FROM transactions
		WHERE status = 'SUCCEEDED'
		  AND processed_at >= $1
		  AND processed_at < $2
		GROUP BY month
		ORDER BY month ASC
	`

	var monthlyRows []struct {
		Month     int   `db:"month"`
		NetAmount int64 `db:"net_amount"`
	}

	if err := r.db.SelectContext(ctx, &monthlyRows, monthlyQuery, start, end); err != nil {
		return nil, fmt.Errorf("failed to aggregate monthly stats: %w", err)
	}

	// Convert to Domain Model
	months := make([]financial.MonthSummary, len(monthlyRows))
	for i, row := range monthlyRows {
		months[i] = financial.MonthSummary{
			Month:  row.Month,
			Amount: row.NetAmount,
		}
	}

	return &financial.AnnualSummaryDTO{
		Year:             year,
		TotalRevenue:     stats.Revenue,
		TotalRefunds:     stats.Refunds,
		NetRevenue:       stats.Revenue - stats.Refunds,
		TransactionCount: stats.Count,
		Currency:         "EUR", // Default or derived from config
		MonthlyBreakdown: months,
	}, nil
}

// --- GDPR ---
func (r *FinancialRepository) AnonymizeUserRecords(ctx context.Context, userID uuid.UUID) error {
	// We cannot delete financial records (transactions/invoices) due to auditing requirements.
	// However, we can remove payment methods which contain PII (last4, brand meta).
	q, a, _ := r.sq.Delete("payment_methods").Where(squirrel.Eq{"user_id": userID}).ToSql()
	_, err := r.runner.ExecContext(ctx, q, a...)
	// We also don't delete the 'customers' mapping table to maintain link to Stripe if audit needed,
	// but we could nullify the user_id if strict anonymity is required, effectively orphaning the record.
	// Strategy: Keep transaction history, but unlink PII.
	return err
}

// To avoid huge wall of text, assume standard mapping for Subscription, Product, Invoice, etc.
type dbSubscription struct {
	ID                 uuid.UUID    `db:"id"`
	OrganizationID     *uuid.UUID   `db:"organization_id"`
	UserID             uuid.UUID    `db:"user_id"`
	PriceID            uuid.UUID    `db:"price_id"`
	Status             string       `db:"status"`
	Provider           string       `db:"provider"`
	ProviderID         string       `db:"provider_id"`
	CurrentPeriodStart time.Time    `db:"current_period_start"`
	CurrentPeriodEnd   time.Time    `db:"current_period_end"`
	CancelledAt        sql.NullTime `db:"cancelled_at"`
	EndedAt            sql.NullTime `db:"ended_at"`
}

func (d *dbSubscription) toDomain() *financial.Subscription {
	var ca, ea *time.Time
	if d.CancelledAt.Valid {
		ca = &d.CancelledAt.Time
	}
	if d.EndedAt.Valid {
		ea = &d.EndedAt.Time
	}
	return &financial.Subscription{
		ID: d.ID, UserID: d.UserID, PriceID: d.PriceID, Status: financial.SubscriptionStatus(d.Status),
		Provider: d.Provider, ProviderID: d.ProviderID, CurrentPeriodStart: d.CurrentPeriodStart, CurrentPeriodEnd: d.CurrentPeriodEnd,
		CancelledAt: ca, EndedAt: ea,
	}
}

type dbProduct struct {
	ID             uuid.UUID  `db:"id"`
	OrganizationID *uuid.UUID `db:"organization_id"`
	Type           string     `db:"type"`
	Name           string     `db:"name"`
	Description    string     `db:"description"`
	IsActive       bool       `db:"is_active"`
	CreatedAt      time.Time  `db:"created_at"`
}

func (d *dbProduct) toDomain(prices []dbPrice) *financial.Product {
	p := &financial.Product{
		ID: d.ID, Type: financial.ProductType(d.Type), Name: d.Name, Description: d.Description, IsActive: d.IsActive, CreatedAt: d.CreatedAt,
	}
	for _, pr := range prices {
		p.Prices = append(p.Prices, *pr.toDomain())
	}
	return p
}

type dbPrice struct {
	ID              uuid.UUID      `db:"id"`
	ProductID       uuid.UUID      `db:"product_id"`
	Amount          int64          `db:"amount"`
	Currency        string         `db:"currency"`
	Interval        sql.NullString `db:"interval"`
	ProviderPriceID string         `db:"provider_price_id"`
	IsActive        bool           `db:"is_active"`
}

func (d *dbPrice) toDomain() *financial.Price {
	var i *financial.Interval
	if d.Interval.Valid {
		v := financial.Interval(d.Interval.String)
		i = &v
	}
	return &financial.Price{
		ID: d.ID, ProductID: d.ProductID, Amount: d.Amount, Currency: d.Currency, Interval: i, ProviderPriceID: d.ProviderPriceID, IsActive: d.IsActive,
	}
}

type dbPaymentMethod struct {
	ID         uuid.UUID `db:"id"`
	UserID     uuid.UUID `db:"user_id"`
	Provider   string    `db:"provider"`
	ProviderID string    `db:"provider_id"`
	Type       string    `db:"type"`
	Details    []byte    `db:"details"`
	IsDefault  bool      `db:"is_default"`
	CreatedAt  time.Time `db:"created_at"`
}

func (d *dbPaymentMethod) toDomain() *financial.PaymentMethod {
	var det map[string]string
	_ = json.Unmarshal(d.Details, &det)
	return &financial.PaymentMethod{
		ID: d.ID, UserID: d.UserID, Provider: d.Provider, ProviderID: d.ProviderID, Type: d.Type, Details: det, IsDefault: d.IsDefault, CreatedAt: d.CreatedAt,
	}
}

type dbCustomer struct {
	ID             uuid.UUID  `db:"id"`
	OrganizationID *uuid.UUID `db:"organization_id"`
	UserID         uuid.UUID  `db:"user_id"`
	Provider       string     `db:"provider"`
	ProviderID     string     `db:"provider_id"`
}

func (d *dbCustomer) toDomain() *financial.Customer {
	return &financial.Customer{ID: d.ID, UserID: d.UserID, Provider: d.Provider, ProviderID: d.ProviderID}
}

type dbInvoice struct {
	ID             uuid.UUID     `db:"id"`
	UserID         uuid.UUID     `db:"user_id"`
	SubscriptionID uuid.NullUUID `db:"subscription_id"`
	TransactionID  uuid.NullUUID `db:"transaction_id"`
	Status         string        `db:"status"`
	AmountDue      int64         `db:"amount_due"`
	Currency       string        `db:"currency"`
	DueDate        time.Time     `db:"due_date"`
	PeriodStart    time.Time     `db:"period_start"`
	PeriodEnd      time.Time     `db:"period_end"`
	PaidAt         sql.NullTime  `db:"paid_at"`
	CreatedAt      time.Time     `db:"created_at"`
	UpdatedAt      time.Time     `db:"updated_at"`
}

func (d *dbInvoice) toDomain() *financial.Invoice {
	var subID, txID *uuid.UUID
	if d.SubscriptionID.Valid {
		subID = &d.SubscriptionID.UUID
	}
	if d.TransactionID.Valid {
		txID = &d.TransactionID.UUID
	}
	var pd *time.Time
	if d.PaidAt.Valid {
		pd = &d.PaidAt.Time
	}
	return &financial.Invoice{
		ID: d.ID, UserID: d.UserID, SubscriptionID: subID, TransactionID: txID, Status: financial.InvoiceStatus(d.Status),
		AmountDue: d.AmountDue, Currency: d.Currency, DueDate: d.DueDate, PeriodStart: d.PeriodStart, PeriodEnd: d.PeriodEnd, PaidAt: pd, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

type dbLedgerAccount struct {
	ID        uuid.UUID `db:"id"`
	Name      string    `db:"name"`
	Type      string    `db:"type"`
	Currency  string    `db:"currency"`
	IsActive  bool      `db:"is_active"`
	CreatedAt time.Time `db:"created_at"`
}

func (d *dbLedgerAccount) toDomain() *financial.Account {
	return &financial.Account{ID: d.ID, Name: d.Name, Type: financial.AccountType(d.Type), Currency: d.Currency, IsActive: d.IsActive, CreatedAt: d.CreatedAt}
}

type dbTransaction struct {
	ID                   uuid.UUID      `db:"id"`
	UserID               uuid.NullUUID  `db:"user_id"`
	Type                 string         `db:"type"`
	Status               string         `db:"status"`
	Amount               int64          `db:"amount"`
	Currency             string         `db:"currency"`
	Description          sql.NullString `db:"description"` // Changed to NullString to be safe
	Provider             string         `db:"provider"`
	ProviderID           string         `db:"provider_id"`
	RelatedTransactionID uuid.NullUUID  `db:"related_transaction_id"`

	// --- ADD THESE FIELDS ---
	ReferenceType sql.NullString `db:"reference_type"`
	ReferenceID   uuid.NullUUID  `db:"reference_id"`
	Metadata      []byte         `db:"metadata"` // JSONB

	OrganizationID       *uuid.UUID     `db:"organization_id"`
	ProcessedAt          time.Time      `db:"processed_at"`
}

func (d *dbTransaction) toDomain() *financial.Transaction {
	var uid *uuid.UUID
	if d.UserID.Valid {
		uid = &d.UserID.UUID
	}
	var relID *uuid.UUID
	if d.RelatedTransactionID.Valid {
		relID = &d.RelatedTransactionID.UUID
	}

	// Map new fields
	var refType *string
	if d.ReferenceType.Valid {
		refType = &d.ReferenceType.String
	}
	var refID *uuid.UUID
	if d.ReferenceID.Valid {
		refID = &d.ReferenceID.UUID
	}

	var meta map[string]interface{}
	if len(d.Metadata) > 0 {
		_ = json.Unmarshal(d.Metadata, &meta)
	}

	return &financial.Transaction{
		ID:                   d.ID,
		UserID:               uid,
		Type:                 financial.TransactionType(d.Type),
		Status:               financial.TransactionStatus(d.Status),
		Amount:               d.Amount,
		Currency:             d.Currency,
		Description:          d.Description.String,
		Provider:             d.Provider,
		ProviderID:           d.ProviderID,
		RelatedTransactionID: relID,
		ReferenceType:        refType,
		ReferenceID:          refID,
		Metadata:             meta,
		ProcessedAt:          d.ProcessedAt,
	}
}
