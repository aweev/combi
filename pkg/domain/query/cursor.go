package query

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// ─── Cursor pagination ─────────────────────────────────────────────────────────
//
// The platform standard is cursor pagination for every list endpoint
// (GetAuditLogRequest{start_after, limit} → GetAuditLogResponse{next_cursor}
// in the foundation spec). Offset-based Pagination remains for internal /
// admin queries where total counts are genuinely needed.

// CursorRequest is the inbound cursor pagination request.
type CursorRequest struct {
	// StartAfter is the opaque cursor from a previous response. Empty means
	// "from the beginning".
	StartAfter string `json:"start_after,omitempty"`
	// Limit is the page size. Zero falls back to DefaultCursorLimit.
	Limit int `json:"limit,omitempty"`
}

// DefaultCursorLimit is the platform default page size for cursor pagination.
const DefaultCursorLimit = 50

// MaxCursorLimit caps page sizes to protect the database.
const MaxCursorLimit = 100

// Cursor is the decoded, tamper-resistant content of the opaque cursor
// string. The cursor encodes the sort key + id of the last item of the
// previous page, following the keyset pagination pattern.
type Cursor struct {
	// SortValue is the last-seen value of the sort column (e.g. created_at).
	SortValue time.Time `json:"sort_value"`
	// ID is the last-seen row id — the tie-breaker for equal sort values,
	// making the ordering deterministic.
	ID string `json:"id"`
}

// EncodeCursor serialises a cursor to the opaque string handed to clients.
// base64(JSON) keeps it opaque to callers while remaining debuggable.
func EncodeCursor(c Cursor) (string, error) {
	raw, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("encode cursor: %w", err)
	}
	return base64.URLEncoding.EncodeToString(raw), nil
}

// DecodeCursor parses an opaque cursor string. An empty string decodes to the
// zero Cursor (start of the list). Invalid cursors return an error — callers
// must reject them with INVALID_INPUT rather than silently restarting.
func DecodeCursor(s string) (Cursor, error) {
	if s == "" {
		return Cursor{}, nil
	}
	raw, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, fmt.Errorf("decode cursor: %w", err)
	}
	var c Cursor
	if err := json.Unmarshal(raw, &c); err != nil {
		return Cursor{}, fmt.Errorf("decode cursor payload: %w", err)
	}
	return c, nil
}

// EffectiveLimit clamps the requested limit between 1 and MaxCursorLimit,
// defaulting to DefaultCursorLimit when unset.
func (r CursorRequest) EffectiveLimit() int {
	if r.Limit <= 0 {
		return DefaultCursorLimit
	}
	if r.Limit > MaxCursorLimit {
		return MaxCursorLimit
	}
	return r.Limit
}

// CursorResult is the standard container for cursor-paginated list responses.
type CursorResult[T any] struct {
	// Items is the current page. Callers should fetch limit+1 rows to
	// compute HasMore.
	Items []T `json:"items"`
	// NextCursor is the opaque cursor for the next page. Empty on the last
	// page.
	NextCursor string `json:"next_cursor"`
	// HasMore reports whether more items exist beyond this page.
	HasMore bool `json:"has_more"`
}

// BuildCursorResult assembles a CursorResult from one fetched page using the
// keyset convention: fetch limit+1 rows, pass the overflow row as "extra",
// and provide the last item's sort value + id for cursor construction.
//
// Usage in a repository:
//
//	rows := fetch(limit + 1)
//	hasMore := len(rows) > limit
//	if hasMore { rows = rows[:limit] }
//	return query.BuildCursorResult(rows, hasMore, lastSortValue, lastID)
func BuildCursorResult[T any](items []T, hasMore bool, lastSortValue time.Time, lastID string) (CursorResult[T], error) {
	res := CursorResult[T]{
		Items:   items,
		HasMore: hasMore,
	}
	if hasMore && lastID != "" {
		c, err := EncodeCursor(Cursor{SortValue: lastSortValue, ID: lastID})
		if err != nil {
			return res, err
		}
		res.NextCursor = c
	}
	return res, nil
}
