package postgres

import (
	"context"

	"github.com/combi/pkg/domain/organization"
	"github.com/Masterminds/squirrel"
)

// ScopeOrg represents an entity that has an organization_id field.
type ScopeOrg interface {
	GetOrganizationID() string
}

// WithTenant adds an organization_id filter to the query if it's present in the context.
func WithTenant(ctx context.Context, builder squirrel.SelectBuilder, alias ...string) squirrel.SelectBuilder {
	return WithTenantCol(ctx, builder, "organization_id", alias...)
}

// WithTenantCol adds a custom organization filter to the query.
func WithTenantCol(ctx context.Context, builder squirrel.SelectBuilder, column string, alias ...string) squirrel.SelectBuilder {
	org := organization.OrgFromContext(ctx)
	if org == nil {
		return builder
	}

	fullCol := column
	if len(alias) > 0 {
		fullCol = alias[0] + "." + column
	}

	return builder.Where(squirrel.Eq{fullCol: org.OrgID})
}

// WithTenantDelete adds an organization_id filter to a delete query.
func WithTenantDelete(ctx context.Context, builder squirrel.DeleteBuilder, alias ...string) squirrel.DeleteBuilder {
	return WithTenantDeleteCol(ctx, builder, "organization_id", alias...)
}

// WithTenantDeleteCol adds a custom organization filter to a delete query.
func WithTenantDeleteCol(ctx context.Context, builder squirrel.DeleteBuilder, column string, alias ...string) squirrel.DeleteBuilder {
	org := organization.OrgFromContext(ctx)
	if org == nil {
		return builder
	}

	fullCol := column
	if len(alias) > 0 {
		fullCol = alias[0] + "." + column
	}

	return builder.Where(squirrel.Eq{fullCol: org.OrgID})
}

// WithTenantUpdate adds an organization_id filter to an update query.
func WithTenantUpdate(ctx context.Context, builder squirrel.UpdateBuilder, alias ...string) squirrel.UpdateBuilder {
	return WithTenantUpdateCol(ctx, builder, "organization_id", alias...)
}

// WithTenantUpdateCol adds a custom organization filter to an update query.
func WithTenantUpdateCol(ctx context.Context, builder squirrel.UpdateBuilder, column string, alias ...string) squirrel.UpdateBuilder {
	org := organization.OrgFromContext(ctx)
	if org == nil {
		return builder
	}

	fullCol := column
	if len(alias) > 0 {
		fullCol = alias[0] + "." + column
	}

	return builder.Where(squirrel.Eq{fullCol: org.OrgID})
}
