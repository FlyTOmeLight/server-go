package cli

import "github.com/FlyTOmeLight/server-go/version"

type Version struct {
	ClientVersion *version.VersionInfo `json:"clientVersion,omitempty" yaml:"clientVersion,omitempty"`
	ServerVersion *version.VersionInfo `json:"serverVersion,omitempty" yaml:"serverVersion,omitempty"`
}
