package election

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp/syntax"
	"strings"
	"testing"
	"time"
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

func TestKubernetesLeaseTimings(t *testing.T) {
	kubeconfig := WithKubeConfigFilePath(testKubeconfig(t))
	for _, tt := range []struct {
		name                string
		options             []K8sElectionOption
		lease, renew, retry time.Duration
	}{
		{name: "defaults", options: nil, lease: 15 * time.Second, renew: 10 * time.Second, retry: 2 * time.Second},
		{
			name: "lease only", options: []K8sElectionOption{WithLeaseDuration(3 * time.Second)},
			lease: 3 * time.Second, renew: 2 * time.Second, retry: 400 * time.Millisecond,
		},
		{name: "explicit timings out of order", options: []K8sElectionOption{
			WithRetryPeriod(time.Second), WithRenewDeadline(4 * time.Second), WithLeaseDuration(6 * time.Second),
		}, lease: 6 * time.Second, renew: 4 * time.Second, retry: time.Second},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e, err := NewK8sLeaderElection("application", append(tt.options, kubeconfig)...)
			if err != nil {
				t.Fatal(err)
			}
			if e.leaseDuration != tt.lease || e.renewDeadline != tt.renew || e.retryPeriod != tt.retry {
				t.Fatalf(
					"timings=%s/%s/%s; want %s/%s/%s",
					e.leaseDuration,
					e.renewDeadline,
					e.retryPeriod,
					tt.lease,
					tt.renew,
					tt.retry,
				)
			}
		})
	}
	for _, tt := range []struct {
		name   string
		option K8sElectionOption
	}{
		{name: "zero lease", option: WithLeaseDuration(0)},
		{name: "shorter than serialized lease", option: WithLeaseDuration(900 * time.Millisecond)},
		{name: "zero renew deadline", option: WithRenewDeadline(0)},
		{name: "zero retry period", option: WithRetryPeriod(0)},
		{name: "renew equals lease", option: WithRenewDeadline(15 * time.Second)},
		{name: "retry violates jitter margin", option: WithRetryPeriod(9 * time.Second)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewK8sLeaderElection("application", kubeconfig, tt.option); err == nil {
				t.Fatal("invalid timings accepted")
			}
		})
	}
}

func TestDefaultElectionIdentityUsesPodNameAndSuffix(t *testing.T) {
	t.Setenv("POD_NAME", "worker-42")
	a, b := newTestK8sLeaderElection(t), newTestK8sLeaderElection(t)
	for _, id := range []string{a.id, b.id} {
		if !strings.HasPrefix(id, "worker-42-") || len(id) != len("worker-42-")+8 {
			t.Fatalf("default identity=%q", id)
		}
	}
	if a.id == b.id {
		t.Fatal("independent electors share a default identity")
	}
	t.Setenv("POD_NAME", "")
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		if id := defaultIdentity(); !strings.HasPrefix(id, hostname+"-") {
			t.Fatalf("default identity=%q; want hostname prefix %q", id, hostname)
		}
	}
	e, err := NewK8sLeaderElection("application", WithKubeConfigFilePath(testKubeconfig(t)), WithIdentify("node"))
	if err != nil || e.id != "node" {
		t.Fatalf("explicit identity: election=%v, error=%v", e, err)
	}
}

func TestPreferredInterfacePattern(t *testing.T) {
	var syntaxErr *syntax.Error
	if ip, err := LookupLocalIP("["); ip != "" || !errors.As(err, &syntaxErr) {
		t.Fatalf("invalid interface pattern returned %q, %v", ip, err)
	}
	if ip := GetLocalIP("["); ip != "" {
		t.Fatalf("GetLocalIP invalid pattern returned %q", ip)
	}
	for _, iface := range []string{"en0", "em0", "eth0", "bond0"} {
		if !defaultNICPattern.MatchString(iface) {
			t.Errorf("default pattern rejected %q", iface)
		}
	}
	for _, iface := range []string{"veth0", "lo", "bridge-bond0"} {
		if defaultNICPattern.MatchString(iface) {
			t.Errorf("default pattern matched %q", iface)
		}
	}
}
