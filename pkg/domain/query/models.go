package query

// Pagination represents standard pagination params.
type Pagination struct {
	Page     int `json:"page" validate:"min=1"`
	PageSize int `json:"page_size" validate:"min=1,max=100"`
}

func (p Pagination) Offset() int {
	if p.Page < 1 {
		return 0
	}
	return (p.Page - 1) * p.PageSize
}

func (p Pagination) Limit() int {
	if p.PageSize < 1 {
		return 20
	} // Default default
	return p.PageSize
}

// Result is the standard container for list responses.
type Result[T any] struct {
	Items []T   `json:"items"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Pages int   `json:"pages"`
}

// Filter allows generic filtering (e.g., "status=active").
type Filter struct {
	Search string                 `json:"search"`
	Sort   string                 `json:"sort"` // e.g. "created_at desc"
	Params map[string]interface{} `json:"params"`
}
