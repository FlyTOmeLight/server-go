package core_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

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
		assert.Nil(t, err)
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
		t.Logf("out: %v", out)
	})
}

func TestNilFieldDecode(t *testing.T) {
	t.Run("NilField", func(t *testing.T) {
		type Request struct {
			OrgID  string  `form:"orgID"`
			UserID *string `form:"userID"`
			Name   *string `form:"name"`
			Value  *int    `form:"value"`
			Count  int     `form:"count"`
		}

		name := "test"
		val := 1
		req := Request{
			OrgID: name,
			Name:  &name,
			Value: &val,
		}
		bytes, _ := json.Marshal(req)

		var mapdata map[string]string
		json.Unmarshal(bytes, &mapdata)

		out := core.FormToStringMap(&req)
		_, ok := out["UserID"]
		assert.False(t, ok)
		t.Logf("out: %v %v", out, mapdata)
	})
}
