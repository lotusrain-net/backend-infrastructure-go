package pagination

const (
	// DefaultSize is used when a requested size is less than one.
	DefaultSize = 20
	// MaxSize is the largest allowed page size.
	MaxSize = 100
)

// Meta describes normalized pagination boundaries and navigation state.
type Meta struct {
	// Page is the one-based requested page number.
	Page int `json:"page"`
	// Size is the normalized number of items per page.
	Size int `json:"size"`
	// Total is the total number of matching items.
	Total int64 `json:"total"`
	// Pages is the total number of available pages.
	Pages int `json:"pages"`
	// HasNext reports whether another page is available.
	HasNext bool `json:"has_next"`
	// HasPrev reports whether a previous page is available.
	HasPrev bool `json:"has_prev"`
}

// Page contains a result slice and its pagination metadata.
type Page[T any] struct {
	// Items is always non-nil so it encodes as [] for an empty result.
	Items []T `json:"items"`
	// Meta describes the normalized page boundaries.
	Meta Meta `json:"meta"`
}

// Normalize clamps page and size to their supported boundaries.
func Normalize(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = DefaultSize
	}
	if size > MaxSize {
		size = MaxSize
	}
	return page, size
}

// NewMeta calculates stable pagination metadata.
func NewMeta(page, size int, total int64) Meta {
	page, size = Normalize(page, size)
	if total < 0 {
		total = 0
	}
	pages := 0
	if total > 0 {
		pageCount := total / int64(size)
		if total%int64(size) != 0 {
			pageCount++
		}
		maximum := int(^uint(0) >> 1)
		if pageCount > int64(maximum) {
			pages = maximum
		} else {
			pages = int(pageCount)
		}
	}
	return Meta{
		Page:    page,
		Size:    size,
		Total:   total,
		Pages:   pages,
		HasNext: page < pages,
		HasPrev: page > 1 && pages > 0,
	}
}

// New constructs a page and guarantees that nil items encode as an empty array.
func New[T any](items []T, page, size int, total int64) Page[T] {
	if items == nil {
		items = make([]T, 0)
	}
	return Page[T]{Items: items, Meta: NewMeta(page, size, total)}
}
