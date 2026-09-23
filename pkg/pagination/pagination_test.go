package pagination_test

import (
	"encoding/json"
	"testing"

	"github.com/lotusrain-net/backend-infrastructure-go/pkg/pagination"
)

func TestNewMetaCalculatesPageBoundaries(t *testing.T) {
	meta := pagination.NewMeta(2, 20, 41)

	if meta.Pages != 3 || !meta.HasNext || !meta.HasPrev {
		t.Fatalf("unexpected meta: %#v", meta)
	}
}

func TestNewClampsNegativeTotalAndEncodesNilItemsAsArray(t *testing.T) {
	page := pagination.New[string](nil, 1, 20, -1)
	if page.Meta.Total != 0 || page.Items == nil {
		t.Fatalf("page = %#v", page)
	}
	encoded, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"items":[],"meta":{"page":1,"size":20,"total":0,"pages":0,"has_next":false,"has_prev":false}}` {
		t.Fatalf("JSON = %s", encoded)
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

func TestNewMetaAvoidsOverflowForMaximumTotal(t *testing.T) {
	const maximumTotal = int64(^uint64(0) >> 1)
	meta := pagination.NewMeta(1, pagination.MaxSize, maximumTotal)
	expected := maximumTotal / pagination.MaxSize
	if maximumTotal%pagination.MaxSize != 0 {
		expected++
	}
	maximumInt := int(^uint(0) >> 1)
	if expected > int64(maximumInt) {
		expected = int64(maximumInt)
	}
	if meta.Pages != int(expected) || meta.Pages <= 0 || !meta.HasNext {
		t.Fatalf("meta=%#v expected pages=%d", meta, expected)
	}
}
