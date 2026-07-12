package pagination

const (
	DefaultSize = 20
	MaxSize     = 100
)

type Meta struct {
	Page    int   `json:"page"`
	Size    int   `json:"size"`
	Total   int64 `json:"total"`
	Pages   int   `json:"pages"`
	HasNext bool  `json:"has_next"`
	HasPrev bool  `json:"has_prev"`
}

type Page[T any] struct {
	Items []T  `json:"items"`
	Meta  Meta `json:"meta"`
}

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

func NewMeta(page, size int, total int64) Meta {
	page, size = Normalize(page, size)
	pages := 0
	if total > 0 {
		pages = int((total + int64(size) - 1) / int64(size))
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

func New[T any](items []T, page, size int, total int64) Page[T] {
	return Page[T]{Items: items, Meta: NewMeta(page, size, total)}
}
