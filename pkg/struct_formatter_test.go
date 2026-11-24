package core_test

import (
	"reflect"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	core "github.com/FlyTOmeLight/server-go/pkg"
	"github.com/FlyTOmeLight/server-go/sdk"
)

func TestStructDecode(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		err := core.StructDecode(nil, nil, "")
		assert.NotNil(t, err)
	})

	t.Run("empty in", func(t *testing.T) {
		var out map[string]any
		err := core.StructDecode(nil, &out, "json")
		require.NoError(t, err)
		require.Empty(t, out)
	})

	t.Run("empty out", func(t *testing.T) {
		in := map[string]string{
			"key1": "value1",
			"key2": "value2",
		}
		err := core.StructDecode(&in, nil, "")
		assert.NotNil(t, err)
	})

	t.Run("empty tag", func(t *testing.T) {
		in := map[string]string{
			"key1": "value1",
			"key2": "value2",
		}
		var out map[string]string
		err := core.StructDecode(&in, &out, "")
		assert.Nil(t, err)
		assert.Equal(t, in, out)
	})

	t.Run("custom tag", func(t *testing.T) {
		var out struct {
			Name string `wire:"key"`
		}
		require.NoError(t, core.StructDecode(map[string]string{"key": "value"}, &out, "wire"))
		require.Equal(t, "value", out.Name)
	})

	t.Run("paging", func(t *testing.T) {
		type Request struct {
			sdk.PagingRequest
			OrgID  string `form:"orgID"`
			UserID string `form:"userID"`
		}

		req := Request{
			PagingRequest: sdk.NewPagingRequest(),
			OrgID:         "test",
			UserID:        "test",
		}
		out := core.FormToStringMap(&req)
		assert.Equal(t, "test", out["orgID"])
		assert.Equal(t, "test", out["userID"])
	})
}

func TestFormToStringMap(t *testing.T) {
	type request struct {
		OrgID  string  `form:"orgID"`
		UserID *string `form:"userID"`
		Name   *string `form:"name"`
		Value  *int    `form:"value"`
		Count  int     `form:"count"`
	}
	var nilRequest *request
	require.Empty(t, core.FormToStringMap(nil))
	require.Empty(t, core.FormToStringMap(nilRequest))

	name := "test"
	value := 1
	req := &request{OrgID: name, Name: &name, Value: &value}
	want := map[string]string{"orgID": "test", "name": "test", "value": "1", "count": "0"}
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			for range 32 {
				if got := core.FormToStringMap(req); !reflect.DeepEqual(got, want) {
					t.Errorf("FormToStringMap = %v, want %v", got, want)
					return
				}
			}
		})
	}
	workers.Wait()
}
