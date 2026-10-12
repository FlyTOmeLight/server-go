package core_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Run the already-built test process from a fresh working directory so import
// initialization happens there, before the helper test itself can run.
func TestImportDoesNotConfigureLogging(t *testing.T) {
	if os.Getenv("SERVER_GO_IMPORT_PROBE") == "1" {
		return
	}
	cwd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cwd, "conf", "logger"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, "conf", "logger", "http.toml"), []byte("not a valid logging config"), 0600); err != nil {
		t.Fatal(err)
	}
	binary, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "-test.run=^TestImportDoesNotConfigureLogging$")
	cmd.Dir = cwd
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "CONSOLE_LOG_LEVEL=") && !strings.HasPrefix(value, "SERVER_GO_IMPORT_PROBE=") {
			cmd.Env = append(cmd.Env, value)
		}
	}
	cmd.Env = append(cmd.Env, "SERVER_GO_IMPORT_PROBE=1", "CONSOLE_LOG_LEVEL=invalid-level")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("import configured logging or failed: %v\n%s", err, output)
	}
	if _, err := os.Stat(filepath.Join(cwd, "logs")); !os.IsNotExist(err) {
		t.Fatalf("import created log output: %v", err)
	}
}
