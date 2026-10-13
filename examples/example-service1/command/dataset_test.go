package command

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/FlyTOmeLight/server-go/cli"
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
	line := cli.NewCommandLineWithName("test")
	config := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(
		config,
		[]byte("core:\n  endpoint: "+server.URL+"\nlog:\n  handler: NONE\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	line.SetConfigFile(config)
	line.Append(&DatasetCommandBuilder{})
	filename := filepath.Join(t.TempDir(), "apply.yaml")
	if err := os.WriteFile(
		filename,
		[]byte(
			"kind: Dataset\nmetadata:\n  projectName: project1\n  localName: dataset1\n  description: applied\n",
		),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	line.Root.SetArgs([]string{"apply", "-f", filename})
	if err := line.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
}
