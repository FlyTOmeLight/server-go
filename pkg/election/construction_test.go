package election

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func testKubeconfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kubeconfig")
	content := `apiVersion: v1
kind: Config
clusters:
- name: cluster
  cluster:
    server: http://127.0.0.1:1
contexts:
- name: current
  context:
    cluster: cluster
    namespace: workers
current-context: current
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func newTestK8sLeaderElection(t *testing.T) *K8sLeaderElection {
	t.Helper()
	e, err := NewK8sLeaderElection("application", WithKubeConfigFilePath(testKubeconfig(t)))
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestKubernetesConstructionNeverFallsBackToLocalLeader(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")
	for _, options := range [][]K8sElectionOption{
		nil,
		{WithKubeConfigFilePath(filepath.Join(t.TempDir(), "missing"))},
		{WithKubeConfigFilePath(testKubeconfig(t)), WithNamespace("Invalid Namespace")},
	} {
		e, err := NewLeaderElection("application", options...)
		if err == nil || e != nil {
			t.Fatalf("invalid Kubernetes configuration returned election=%T, error=%v", e, err)
		}
	}
}

func TestKubeconfigNamespaceAndExplicitOverride(t *testing.T) {
	for _, namespace := range []string{"", "override"} {
		e, err := NewK8sLeaderElection(
			"application",
			WithKubeConfigFilePath(testKubeconfig(t)),
			WithNamespace(namespace),
		)
		if err != nil {
			t.Fatal(err)
		}
		want := namespace
		if want == "" {
			want = "workers"
		}
		if e.namespace != want || e.IsLeader() {
			t.Fatalf("namespace=%q leader=%v", e.namespace, e.IsLeader())
		}
	}
}

func TestAsyncElectionReportsCanceledStartup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := <-newTestK8sLeaderElection(t).RunAsync(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled startup returned %v", err)
	}
}
