package sdk

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPagingRequest_IsPage(t *testing.T) {
	testcases := []struct {
		desc   string
		paging PagingRequest
		want   bool
	}{
		{
			desc: "good",
			paging: PagingRequest{
				PageNo:   1,
				PageSize: 20,
			},
			want: true,
		},
		{
			desc: "bad",
			want: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			actual := tc.paging.IsPage()
			assert.Equal(t, tc.want, actual)
		})
	}
}

func TestPagingRequest_GetPageNo(t *testing.T) {
	testcases := []struct {
		desc   string
		paging PagingRequest
		want   int
	}{
		{
			desc: "normal",
			paging: PagingRequest{
				PageNo:   1,
				PageSize: 20,
			},
			want: 1,
		},
		{
			desc: "min",
			want: 1,
		},
		{desc: "negative", paging: PagingRequest{PageNo: -1}, want: 1},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			actual := tc.paging.GetPageNo()
			assert.Equal(t, tc.want, actual)
		})
	}
}

func TestPagingRequest_GetPageSize(t *testing.T) {
	testcases := []struct {
		desc   string
		paging PagingRequest
		want   int
	}{
		{
			desc: "normal",
			paging: PagingRequest{
				PageNo:   1,
				PageSize: 10,
			},
			want: 10,
		},
		{
			desc: "min",
			want: 20,
		},
		{desc: "negative", paging: PagingRequest{PageSize: -1}, want: 20},
		{
			desc: "max",
			paging: PagingRequest{
				PageSize: 200,
			},
			want: 200,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			actual := tc.paging.GetPageSize()
			assert.Equal(t, tc.want, actual)
		})
	}
}

func TestPagingRequest_Offset(t *testing.T) {
	paging := PagingRequest{}
	actual := paging.Offset()
	assert.Equal(t, 0, actual)
}

func TestPagingRequest_Limit(t *testing.T) {
	paging := PagingRequest{}
	actual := paging.Limit()
	assert.Equal(t, 20, actual)
}

func TestPagingRequest_IsOrder(t *testing.T) {
	testcases := []struct {
		desc   string
		paging PagingRequest
		want   bool
	}{
		{
			desc: "good",
			paging: PagingRequest{
				OrderBy: "name",
			},
			want: true,
		},
		{
			desc: "bad",
			want: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			actual := tc.paging.IsOrder()
			assert.Equal(t, tc.want, actual)
		})
	}
}

func TestPagingRequest_GetOrder(t *testing.T) {
	testcases := []struct {
		desc   string
		paging PagingRequest
		want   string
	}{
		{
			desc: "default",
			want: "desc",
		},
		{
			desc: "asc",
			paging: PagingRequest{
				Order: OrderASC,
			},
			want: "asc",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			actual := tc.paging.GetOrder()
			assert.Equal(t, tc.want, actual)
		})
	}
}

func TestPagingRequest_DBOrder(t *testing.T) {
	for _, tc := range []struct {
		name   string
		paging PagingRequest
		want   string
	}{
		{name: "default", want: "created_at desc"},
		{name: "default ascending", paging: PagingRequest{Order: OrderASC}, want: "created_at asc"},
		{name: "snake case", paging: PagingRequest{OrderBy: "updated_at"}, want: "updated_at desc"},
		{name: "camel case", paging: PagingRequest{OrderBy: "updatedAt", Order: OrderASC}, want: "updated_at asc"},
		{name: "punctuation stripped", paging: PagingRequest{OrderBy: "updated--At!"}, want: "updated_at desc"},
		{name: "empty after stripping", paging: PagingRequest{OrderBy: "--!", Order: OrderASC}, want: "created_at asc"},
		{name: "unknown direction", paging: PagingRequest{OrderBy: "name", Order: "ASC"}, want: "name desc"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.paging.DBOrder())
		})
	}
}

func TestPagingRequest_DBOrderWithAllowlist(t *testing.T) {
	allowed := map[string]string{"updatedAt": "records.updated_at", "empty": ""}
	for _, tc := range []struct {
		name          string
		paging        PagingRequest
		defaultColumn string
		want          string
		wantErr       bool
	}{
		{name: "mapped", paging: PagingRequest{OrderBy: "updatedAt", Order: OrderASC}, want: "records.updated_at asc"},
		{name: "default", defaultColumn: "records.id", want: "records.id desc"},
		{name: "built-in default", paging: PagingRequest{Order: OrderASC}, want: "created_at asc"},
		{name: "unknown", paging: PagingRequest{OrderBy: "updated_at"}, defaultColumn: "records.id", wantErr: true},
		{name: "requires exact key", paging: PagingRequest{OrderBy: "updated--At"}, wantErr: true},
		{name: "empty mapping", paging: PagingRequest{OrderBy: "empty"}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.paging.DBOrderWithAllowlist(allowed, tc.defaultColumn)
			assert.Equal(t, tc.want, got)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPaging(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	for _, tc := range []struct {
		name     string
		input    any
		page     int
		pageSize int
		want     []any
	}{
		{name: "slice", input: items, page: 2, pageSize: 2, want: []any{3, 4}},
		{name: "partial page", input: items, page: 3, pageSize: 2, want: []any{5}},
		{name: "array", input: [3]int{1, 2, 3}, page: 2, pageSize: 1, want: []any{2}},
		{name: "string bytes", input: "abc", page: 2, pageSize: 1, want: []any{byte('b')}},
		{name: "page zero", input: items, pageSize: 2, want: []any{1, 2}},
		{name: "negative page", input: items, page: math.MinInt, pageSize: 2, want: []any{1, 2}},
		{name: "offset at length", input: items, page: 2, pageSize: 5, want: []any{}},
		{name: "offset past length", input: items, page: 4, pageSize: 2, want: []any{}},
		{name: "offset overflow", input: items, page: math.MaxInt/2 + 2, pageSize: 2, want: []any{}},
		{name: "huge page size", input: items, page: 1, pageSize: math.MaxInt, want: []any{1, 2, 3, 4, 5}},
		{name: "zero size", input: items, page: 1, want: []any{}},
		{name: "negative size", input: items, page: 1, pageSize: -1, want: []any{}},
		{name: "nil", page: 1, pageSize: 2, want: []any{}},
		{name: "nil slice", input: []int(nil), page: 1, pageSize: 2, want: []any{}},
		{name: "map", input: map[string]int{"a": 1}, page: 1, pageSize: 2, want: []any{}},
		{name: "scalar", input: 1, page: 1, pageSize: 2, want: []any{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Paging(tc.input, tc.page, tc.pageSize))
		})
	}
}
