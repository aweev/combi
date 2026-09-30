package envelope

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/combi/pkg/apperror"
	"github.com/combi/pkg/errors"
	"github.com/combi/pkg/middleware"
)

// Builder assembles an Envelope with chained calls. Start with New(ctx, data).
type Builder struct {
	env        Envelope
	httpStatus int
}

// New starts a success envelope for the given data. Correlation values are
// pulled from the request context (request id, tenant id).
func New(ctx context.Context, data any) *Builder {
	return &Builder{
		env: Envelope{
			Data: data,
			Meta: Meta{
				RequestID: middleware.RequestIDFromContext(ctx),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Version:   APIVersion,
				TenantID:  middleware.TenantIDFromContext(ctx),
			},
			Events: []EventRef{},
		},
		httpStatus: http.StatusOK,
	}
}

// WithEvent appends an event reference emitted by this request.
func (b *Builder) WithEvent(eventType, id string) *Builder {
	b.env.Events = append(b.env.Events, EventRef{Type: eventType, ID: id})
	return b
}

// WithLink adds a single HATEOAS link.
func (b *Builder) WithLink(rel, href string) *Builder {
	if b.env.Links == nil {
		b.env.Links = map[string]string{}
	}
	b.env.Links[rel] = href
	return b
}

// WithLinks adds multiple HATEOAS links at once.
func (b *Builder) WithLinks(links map[string]string) *Builder {
	for rel, href := range links {
		b = b.WithLink(rel, href)
	}
	return b
}

// WithPagination attaches cursor pagination metadata for list responses.
func (b *Builder) WithPagination(hasMore bool, nextCursor string, pageSize int) *Builder {
	b.env.Meta.Pagination = &Pagination{
		HasMore:    hasMore,
		NextCursor: nextCursor,
		PageSize:   pageSize,
	}
	return b
}

// Envelope returns the assembled envelope without writing it.
func (b *Builder) Envelope() Envelope {
	return b.env
}

// Write serialises the envelope as JSON and writes it with the builder's
// status (200 for success builders, the mapped error status for error ones).
func (b *Builder) Write(w http.ResponseWriter) error {
	return b.env.Write(w, b.httpStatus)
}

// Write serialises the envelope as JSON and writes it with the given status.
func (e Envelope) Write(w http.ResponseWriter, status int) error {
	body, err := json.Marshal(e)
	if err != nil {
		http.Error(w, `{"error":{"code":"INTERNAL","message":"response serialisation failed"}}`, http.StatusInternalServerError)
		return err
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, err = w.Write(body)
	return err
}

// ─── Error responses ──────────────────────────────────────────────────────────

// NewError builds an error envelope from a canonical AfrosError. The error
// block uses the same HTTPError shape as AfrosError.ToHTTP().
func NewError(ctx context.Context, err error) *Builder {
	requestID := middleware.RequestIDFromContext(ctx)
	traceID := middleware.TraceIDFromContext(ctx)

	// Normalise anything into a canonical AfrosError first: canonical errors
	// pass through, legacy AppErrors are bridged, anything else becomes
	// INTERNAL without leaking internals.
	afros := errors.FromError(err)
	if afros == nil {
		afros = apperror.AsAfros(err)
	}
	if afros == nil {
		afros = errors.Internal("an unexpected error occurred")
	}
	afros = afros.WithRequest(requestID).WithTrace(traceID)

	httpStatus, body := afros.ToHTTP()
	return &Builder{
		env: Envelope{
			Data: nil,
			Meta: Meta{
				RequestID: requestID,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Version:   APIVersion,
				TenantID:  middleware.TenantIDFromContext(ctx),
			},
			Error: &body,
		},
		httpStatus: httpStatus,
	}
}

// WriteError writes a complete error response for the given error, mapping
// it to the correct HTTP status via the canonical error system.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	NewError(r.Context(), err).Write(w)
}
