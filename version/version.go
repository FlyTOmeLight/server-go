package version

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

var (
	AppVersion    = "unknown"
	GitBranch     = "unknown"
	GitCommit     = "unknown"
	BuildDate     = "unknown"
	BuildPipeline = "unknown"
	BuildNumber   = "unknown"
)

type VersionInfo struct {
	AppVersion    string
	GitBranch     string
	GitCommit     string
	BuildDate     string
	BuildPipeline string
	BuildNumber   string
}

func Version() core.HandlerFunc {
	return func(c *core.Context) {
		c.JSON(http.StatusOK, GetVersion())
	}
}

func GetVersionString() string {
	bytes, _ := json.Marshal(GetVersion())
	return string(bytes)
}

func GetVersion() *VersionInfo {
	versionInfo := VersionInfo{
		AppVersion:    AppVersion,
		GitBranch:     GitBranch,
		GitCommit:     GitCommit,
		BuildDate:     BuildDate,
		BuildPipeline: BuildPipeline,
		BuildNumber:   BuildNumber,
	}
	return &versionInfo
}

func GetVersionByURL(endpoint string) (*VersionInfo, error) {
	return GetVersionByURLContext(context.Background(), endpoint)
}

// GetVersionByURLContext fetches the version from the endpoint using ctx.
func GetVersionByURLContext(ctx context.Context, endpoint string) (*VersionInfo, error) {
	endpoint = strings.TrimSpace(endpoint)
	if strings.TrimRight(endpoint, "/") == "" {
		return nil, errors.New("version endpoint is empty")
	}
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "http://" + endpoint
	}
	endpoint = strings.TrimRight(endpoint, "/")
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/version", nil)
	if err != nil {
		return nil, err
	}
	if request.URL.Host == "" {
		return nil, errors.New("version endpoint has no host")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("version endpoint returned HTTP %d", response.StatusCode)
	}

	var info VersionInfo
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}
