package host

import (
	"net/http"

	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/httpintegration"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

// AdminAddress returns the bound address of the optional admin listener.
func (h *HTTPHost) AdminAddress() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.adminAddress
}

func (h *HTTPHost) buildAdminRouter(config *conf.Config) (*core.Engine, error) {
	engine, err := h.newRouter(config, nil)
	if err != nil {
		return nil, err
	}
	httpintegration.InstallDiagnostics(engine, config.HTTPServer)
	return engine, nil
}

func (h *HTTPHost) newAdminServer(config conf.HTTPServerConfig) *http.Server {
	config.Listen = config.AdminListen
	config.TLS = conf.HTTPServerTLSConfig{}
	config.UseH2C = false
	server := newHTTPServer(config)
	server.Handler = h.serveRouter(&h.adminRouter)
	return server
}
