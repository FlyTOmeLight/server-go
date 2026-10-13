package httpintegration

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/auth"
	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/constants"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

func TestInstallLeavesDisabledFeaturesAndDaprUnregistered(t *testing.T) {
	engine := gin.New()
	mode := gin.Mode()
	if err := Install(engine, &conf.Config{RunMode: conf.RunModeTest}); err != nil {
		t.Fatal(err)
	}
	if gin.Mode() != mode {
		t.Fatal("Install changed process-wide Gin mode")
	}
	routes := engine.Routes()
	if len(routes) != 1 || routes[0].Path != "/feature" {
		t.Fatalf("disabled optional features registered routes: %v", routes)
	}
}

func TestInstallEnablesConfiguredRoutesAndProtectsAssets(t *testing.T) {
	assets := t.TempDir()
	if err := os.WriteFile(filepath.Join(assets, "file.txt"), []byte("asset"), 0o600); err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	identities := make(map[string]string)
	engine.Use(func(c *core.Context) {
		c.Next()
		identities[c.Request.URL.Path] = c.GetString(constants.KeyUserID)
	})
	config := &conf.Config{
		AppName: "example", AppCode: 1, RunMode: conf.RunModeRelease,
		HTTPServer: conf.HTTPServerConfig{
			PprofEnable: true, MetricsEnable: true, SwaggerEnable: true, ConsoleEnable: true,
			ConsoleProxyNames: []string{"other"}, AuthType: conf.AuthTypeFake,
			Assets: []conf.HTTPServerAssetConfig{
				{URLPath: "/public", FilePath: assets},
				{URLPath: "/protected", FilePath: assets, AuthEnable: true},
			},
		},
		FakeAuth: conf.FakeAuthConfig{UserID: "configured-user"},
	}
	if err := Install(engine, config); err != nil {
		t.Fatal(err)
	}
	engine.GET("/business", func(c *core.Context) { c.String(http.StatusOK, c.GetString(constants.KeyUserID)) })
	paths := map[string]bool{}
	for _, route := range engine.Routes() {
		key := route.Method + " " + route.Path
		if paths[key] {
			t.Fatalf("duplicate route: %s", key)
		}
		paths[key] = true
	}
	for _, route := range []string{"GET /metrics", "GET /pprof/", "GET /swagger/*any", "GET /feature", "GET /example/api/*any", "POST /example/api/*any", "GET /other/api/*any"} {
		if !paths[route] {
			t.Errorf("missing configured route: %s", route)
		}
	}
	for _, test := range []struct {
		path string
		user string
	}{
		{path: "/public/file.txt"},
		{path: "/protected/file.txt", user: "configured-user"},
		{path: "/business", user: "configured-user"},
	} {
		writer := httptest.NewRecorder()
		engine.ServeHTTP(writer, httptest.NewRequest(http.MethodGet, test.path, nil))
		if writer.Code != http.StatusOK || identities[test.path] != test.user {
			t.Errorf("route %s: status %d identity %q", test.path, writer.Code, identities[test.path])
		}
	}
}

func TestInstallRejectsInvalidInputs(t *testing.T) {
	for _, test := range []struct {
		engine *core.Engine
		config *conf.Config
	}{
		{config: &conf.Config{}},
		{engine: gin.New()},
		{engine: gin.New(), config: &conf.Config{HTTPServer: conf.HTTPServerConfig{AuthType: "unsupported"}}},
		{engine: gin.New(), config: &conf.Config{HTTPServer: conf.HTTPServerConfig{AuthType: conf.AuthTypeAuthCenter}}},
	} {
		if err := Install(test.engine, test.config); err == nil {
			t.Fatal("expected invalid integration configuration error")
		}
	}
}

func TestInstallWithVerifiedAuthentication(t *testing.T) {
	config := &conf.Config{HTTPServer: conf.HTTPServerConfig{AuthType: conf.AuthTypeAuthCenter}}
	engine := gin.New()
	if err := Install(engine, config); err == nil || len(engine.Routes()) != 0 {
		t.Fatal("unverified AuthCenter must fail before registering routes")
	}
	handler, err := auth.TrustedIdentityHeaders(func(r *http.Request) error {
		if r.Header.Get("Authorization") == "Bearer gateway-secret" {
			return nil
		}
		return os.ErrPermission
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := InstallWithAuth(engine, config, handler); err != nil {
		t.Fatal(err)
	}
	engine.GET("/business", func(c *core.Context) { c.String(http.StatusOK, auth.UserID(c)) })
	for _, token := range []string{"", "gateway-secret"} {
		request := httptest.NewRequest(http.MethodGet, "/business", nil)
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set(constants.HeaderOrgID, "org")
		request.Header.Set(constants.HeaderUserID, "user")
		writer := httptest.NewRecorder()
		engine.ServeHTTP(writer, request)
		if token == "" && writer.Code != http.StatusUnauthorized {
			t.Fatal("untrusted identity accepted")
		}
		if token != "" && (writer.Code != http.StatusOK || writer.Body.String() != "user") {
			t.Fatal("trusted identity not installed on business routes")
		}
	}
}
