package sdk

import (
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strings"
)

var (
	re            = regexp.MustCompile(`[^a-zA-Z0-9_]`)
	matchFirstCap = regexp.MustCompile(`(.)([A-Z][a-z]+)`)
	matchAllCap   = regexp.MustCompile(`([a-z0-9])([A-Z])`)
)

const (
	OrderASC  = "asc"
	OrderDESC = "desc"
)

const (
	minPageNo       = 1
	maxPageSize     = 200
	defaultPageSize = 20
	defaultOrderBy  = "created_at"
)

// swagger:model PagingResponse
type PagingResponse struct {
	Results    any `json:"result"`
	TotalCount int `json:"totalCount"`
}

func NewPagingRequest() PagingRequest {
	return PagingRequest{
		PageNo:   minPageNo,
		PageSize: defaultPageSize,
	}
}

// PagingRequest 通用分页参数
// swagger:model PagingRequest
type PagingRequest struct {
	// 页码
	//
	// Example: 1
	PageNo int `json:"pageNo" form:"pageNo" binding:"min=0" default:"1"`

	// 每页的大小
	//
	// Example: 10
	PageSize int `json:"pageSize" form:"pageSize" binding:"min=0,max=200" default:"20"`

	// 排序规则: asc: 升序, desc: 倒序, 默认倒序
	//
	// Example: desc
	Order string `json:"order" form:"order" binding:"omitempty,oneof=asc desc" default:"desc"`

	// 排序依据
	//
	// Example: name
	OrderBy string `json:"orderBy" form:"orderBy" binding:"orderby"`

	// mongo 排序规则
	//
	// Example: []string{ "{order: desc}", "{id:asc}" }
	Orders []string `json:"orders" form:"orders" example:"[{create_ad:desc},{id:asc}]"`
}

func (r PagingRequest) IsPage() bool {
	return r.PageNo > 0 && r.PageSize > 0
}

func (r PagingRequest) GetPageNo() int {
	if r.PageNo < minPageNo {
		return minPageNo
	}
	return r.PageNo
}

func (r PagingRequest) GetPageSize() int {
	if r.PageSize < 1 {
		return defaultPageSize
	}
	if r.PageSize > maxPageSize {
		return maxPageSize
	}
	return r.PageSize
}

func (r PagingRequest) Offset() int {
	return (r.GetPageNo() - 1) * r.GetPageSize()
}

func (r PagingRequest) Limit() int {
	return r.GetPageSize()
}

func (r PagingRequest) IsOrder() bool {
	return r.OrderBy != ""
}

func (r PagingRequest) GetOrder() string {
	if r.Order == OrderASC {
		return OrderASC
	}
	return OrderDESC
}

// DBOrder 数据库排序语句
func (r PagingRequest) DBOrder() string {
	column := re.ReplaceAllString(r.OrderBy, "")
	if column == "" {
		return defaultOrderBy + " " + r.GetOrder()
	}
	return camelToSnake(column) + " " + r.GetOrder()
}

// DBOrderWithAllowlist maps OrderBy to a database column. The caller supplies
// trusted SQL column names in allowed and defaultColumn.
func (r PagingRequest) DBOrderWithAllowlist(allowed map[string]string, defaultColumn string) (string, error) {
	if r.OrderBy == "" {
		if defaultColumn == "" {
			defaultColumn = defaultOrderBy
		}
		return defaultColumn + " " + r.GetOrder(), nil
	}

	column := allowed[r.OrderBy]
	if column == "" {
		return "", fmt.Errorf("unknown order column %q", r.OrderBy)
	}
	return column + " " + r.GetOrder(), nil
}

// Paging 对切片、数组或字符串做分页处理。
func Paging(arr any, pageNo int, pageSize int) []any {
	if pageSize <= 0 {
		return []any{}
	}

	value := reflect.ValueOf(arr)
	switch value.Kind() {
	case reflect.Slice, reflect.Array, reflect.String:
	default:
		return []any{}
	}

	pageIndex := max(pageNo, minPageNo) - 1
	if pageIndex > math.MaxInt/pageSize {
		return []any{}
	}
	offset := pageIndex * pageSize
	length := value.Len()
	if offset >= length {
		return []any{}
	}

	count := min(pageSize, length-offset)
	paged := make([]any, count)
	for i := range paged {
		paged[i] = value.Index(offset + i).Interface()
	}
	return paged
}

// 将 api 中的驼峰字段转化为 db 下划线字段形式
func camelToSnake(s string) string {
	snake := matchFirstCap.ReplaceAllString(s, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}
