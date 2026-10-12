package auth

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/constants"
)

func TestDefaultFakeAuthIsSafeForConcurrentRequests(t *testing.T) {
	engine := gin.New()
	engine.Use(FakeAuth())
	engine.GET("/identity", func(c *gin.Context) {
		for key, want := range map[string]string{
			constants.KeyOrgID: "test-org-id", constants.KeyUserID: "test-user-id",
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
