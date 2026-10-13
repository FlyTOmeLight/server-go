package trainv1

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	frameworkerrors "github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/sdk"
)

func TestDatasetClientCancellationAndHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/projects/project/datasets/data" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}))
	defer server.Close()
	client, err := NewClientWithConfig(&sdk.Config{Endpoint: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	req := &GetDatasetRequest{ProjectName: "project", LocalName: "data"}
	_, err = client.GetDataset(t.Context(), req)
	var detail *frameworkerrors.Error
	if !errors.As(err, &detail) || detail.Status != http.StatusBadGateway {
		t.Fatalf("HTTP failure: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err = client.GetDataset(ctx, req); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}
