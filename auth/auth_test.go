package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/constants"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

func TestUseRejectsFakeAuthenticationInReleaseMode(t *testing.T) {
	for _, authType := range []conf.AuthType{"", conf.AuthTypeFake} {
		engine := gin.New()
		config := &conf.Config{
			RunMode: conf.RunModeRelease, HTTPServer: conf.HTTPServerConfig{AuthType: authType},
		}
		if err := Use(engine, config); err == nil || !strings.Contains(err.Error(), "release mode") {
			t.Fatalf("AuthType %q: expected release authentication error, got %v", authType, err)
		}
		engine.GET("/", func(c *core.Context) {
			if UserID(c) != "" {
				t.Error("fake identity installed after rejection")
			}
		})
		engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	}
}

func TestDefaultFakeAuthIsSafeForConcurrentRequests(t *testing.T) {
	engine := gin.New()
	engine.Use(FakeAuth())
	engine.GET("/identity", func(c *core.Context) {
		for key, want := range map[string]string{
			constants.KeyTenantID: "test-tenant-id", constants.KeyUserID: "test-user-id",
			constants.KeyUserName: "test-user-name", constants.KeyUserRole: "test-user-role",
		} {
			if got := c.GetString(key); got != want {
				t.Errorf("identity %s = %q, want %q", key, got, want)
			}
		}
		c.Status(http.StatusNoContent)
	})
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			writer := httptest.NewRecorder()
			engine.ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "/identity", nil))
			if writer.Code != http.StatusNoContent {
				t.Errorf("identity handler did not run: %d", writer.Code)
			}
		}()
	}
	wg.Wait()
}
