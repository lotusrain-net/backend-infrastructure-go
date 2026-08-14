package pagination_test

import (
	"testing"

	"github.com/jyysy/backend-infrastructure-go/pkg/pagination"
)

func TestNewMetaCalculatesPageBoundaries(t *testing.T) {
	meta := pagination.NewMeta(2, 20, 41)

	if meta.Pages != 3 || !meta.HasNext || !meta.HasPrev {
		t.Fatalf("unexpected meta: %#v", meta)
	}
}

func TestNewMetaHandlesEmptyResult(t *testing.T) {
	meta := pagination.NewMeta(1, 20, 0)

	if meta.Pages != 0 || meta.HasNext || meta.HasPrev {
		t.Fatalf("unexpected empty meta: %#v", meta)
	}
}

func TestNormalizeBoundsPageAndSize(t *testing.T) {
	page, size := pagination.Normalize(-1, 1000)
	if page != 1 || size != pagination.MaxSize {
		t.Fatalf("got page=%d size=%d", page, size)
	}
}
