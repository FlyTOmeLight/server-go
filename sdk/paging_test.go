package sdk

import (
	"github.com/stretchr/testify/assert"
	"testing"
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
	paging := PagingRequest{
		Order:   OrderDESC,
		OrderBy: "created_at",
	}
	actual := paging.DBOrder()
	assert.Equal(t, "created_at desc", actual)
}
