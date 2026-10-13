package command

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FlyTOmeLight/server-go/cli"
	"github.com/FlyTOmeLight/server-go/sdk"
)

func TestDatasetApplyMetadataPreservesProjectPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/projects/project1/datasets" {
			t.Errorf("apply request = %s %s", r.Method, r.URL)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["localName"] != "dataset1" || body["description"] != "applied" {
			t.Errorf("apply body = %#v", body)
		}
		for _, key := range []string{"projectName", "ProjectName", "fileFormat"} {
			if _, found := body[key]; found {
				t.Errorf("path/default field %s leaked into body: %#v", key, body)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"projects/project1/datasets/dataset1","fileFormat":"File"}`))
	}))
	defer server.Close()
	frameworkClient, err := sdk.NewClientWithConfig(&sdk.Config{Endpoint: server.URL, Timeout: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer frameworkClient.Close()
	previousConfig, previousClient := cli.Config, client
	t.Cleanup(func() { cli.Config, client = previousConfig, previousClient })
	cli.Config.Client = frameworkClient
	applyDataset()
	handler, err := cli.GetApplyHandler("Dataset")
	if err != nil {
		t.Fatal(err)
	}
	if err := handler.Handle("Dataset", map[string]any{
		"projectName": "project1", "localName": "dataset1", "description": "applied",
	}); err != nil {
		t.Fatal(err)
	}
}
