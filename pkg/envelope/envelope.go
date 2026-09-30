// Package envelope implements the standard Africa OS REST response envelope.
//
// Every REST response — success or error — uses this shape (from the
// foundation services spec):
//
//	{
//	  "data":   { ... },
//	  "meta":   { "request_id": "req_...", "timestamp": "...", "version": "2026-01", "tenant_id": "ten_..." },
//	  "error":  null,
//	  "events": [{ "type": "identity.created", "id": "idn_..." }],
//	  "_links": { "self": "/identities/idn_...", "verify": "/identities/idn_.../verify" }
//	}
//
// Handlers assemble the envelope with the builder:
//
//	env := envelope.New(req.Context(), identity).
//	    WithEvent("identity.created", idn.ID.String()).
//	    WithLink("self", "/identities/"+idn.ID.String()).
//	    WithLink("verify", "/identities/"+idn.ID.String()+"/verify")
//	env.Write(w, http.StatusOK)
//
// Meta is filled from the request context (request_id, tenant_id) via
// pkg/middleware — tenant data comes only from verified JWT claims, never
// from request bodies.
package envelope

import "github.com/combi/pkg/errors"

// APIVersion is the platform API version carried in meta.version.
const APIVersion = "2026-01"

// Envelope is the standard response wrapper. `error` is nil on success;
// `data` is nil on error.
type Envelope struct {
	Data   any               `json:"data"`
	Meta   Meta              `json:"meta"`
	Error  *errors.HTTPError `json:"error"`
	Events []EventRef        `json:"events"`
	Links  map[string]string `json:"_links,omitempty"`
}

// Meta carries request correlation data on every response.
type Meta struct {
	RequestID  string      `json:"request_id"`
	Timestamp  string      `json:"timestamp"`
	Version    string      `json:"version"`
	TenantID   string      `json:"tenant_id,omitempty"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

// Pagination describes a cursor-paginated list inside meta.
type Pagination struct {
	// HasMore is true when a next cursor exists.
	HasMore bool `json:"has_more"`
	// NextCursor is the opaque cursor to fetch the next page. Empty on the
	// last page.
	NextCursor string `json:"next_cursor,omitempty"`
	// PrevCursor is the opaque cursor to fetch the previous page, when the
	// underlying store supports backward pagination.
	PrevCursor string `json:"prev_cursor,omitempty"`
	// PageSize echoes the effective page size.
	PageSize int `json:"page_size,omitempty"`
}

// EventRef is a lightweight reference to a domain event emitted by the
// request, so clients can track what happened without subscribing to the bus.
type EventRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}
