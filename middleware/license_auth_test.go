package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

type testLicense struct{ authorized bool }

func (l testLicense) GetAuthorization() bool { return l.authorized }

func (l testLicense) ErrorMessage() string { return "not authorized" }

func TestLicenseAuth(t *testing.T) {
	for _, tc := range []struct {
		name   string
		client LicenseAuthorizer
		status int
	}{
		{name: "missing client", client: nil, status: http.StatusInternalServerError},
		{name: "denied", client: testLicense{authorized: false}, status: http.StatusUnauthorized},
		{name: "authorized", client: testLicense{authorized: true}, status: http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := gin.New()
			engine.Use(LicenseAuth(tc.client))
			engine.GET("/", func(c *core.Context) { c.Status(http.StatusNoContent) })
			result := httptest.NewRecorder()
			engine.ServeHTTP(result, httptest.NewRequest(http.MethodGet, "/", nil))
			if result.Code != tc.status {
				t.Fatalf("status = %d, want %d", result.Code, tc.status)
			}
		})
	}
}
