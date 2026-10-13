package middleware

import (
	"github.com/FlyTOmeLight/server-go/conf"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

func Asset(assets []conf.HTTPServerAssetConfig, authEnable bool, engine *core.Engine) {
	for _, asset := range assets {
		if asset.AuthEnable != authEnable {
			continue
		}
		if asset.FilePath == "" {
			continue
		}
		if asset.URLPath == "" {
			asset.URLPath = "/assets"
		}
		engine.Static(asset.URLPath, asset.FilePath)
	}
}
