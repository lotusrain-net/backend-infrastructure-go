// Package pagination is a compatibility adapter for pkg/pagination.
package pagination

import publicpagination "github.com/lotusrain-net/backend-infrastructure-go/pkg/pagination"

const (
	DefaultSize = publicpagination.DefaultSize
	MaxSize     = publicpagination.MaxSize
)

type Meta = publicpagination.Meta
type Page[T any] struct {
	Items []T  `json:"items"`
	Meta  Meta `json:"meta"`
}

func Normalize(page, size int) (int, int) { return publicpagination.Normalize(page, size) }
func NewMeta(page, size int, total int64) Meta {
	return publicpagination.NewMeta(page, size, total)
}
func New[T any](items []T, page, size int, total int64) Page[T] {
	result := publicpagination.New(items, page, size, total)
	return Page[T]{Items: result.Items, Meta: result.Meta}
}
