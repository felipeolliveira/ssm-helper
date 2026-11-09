package main

import (
	_ "embed"
	"encoding/json"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/felipeolliveira/ssm-helper/internal/tunnel"
)

// wailsConfig is embedded so the About screen shows the same version as the
// .exe properties: the release workflow writes the tag's version into
// wails.json before building.
//
//go:embed wails.json
var wailsConfig []byte

// AboutInfo is what the About screen shows.
type AboutInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Copyright   string `json:"copyright"`
	// Commit is the Git revision of the build; empty when Go did not record it.
	Commit     string `json:"commit"`
	CommitTime string `json:"commitTime"`
	// Modified means the build had uncommitted changes.
	Modified bool `json:"modified"`
	// BuildType is "production", "dev" or "debug".
	BuildType     string `json:"buildType"`
	Platform      string `json:"platform"`
	GoVersion     string `json:"goVersion"`
	WailsVersion  string `json:"wailsVersion"`
	PluginVersion string `json:"pluginVersion"`
	Repository    string `json:"repository"`
	AppDir        string `json:"appDir"`
	DataDir       string `json:"dataDir"`
	LogDir        string `json:"logDir"`
}

// About returns the version of the program and of what it is built with.
func (a *App) About() AboutInfo {
	info := AboutInfo{
		Name:          "SSM Helper",
		Platform:      runtime.GOOS + "/" + runtime.GOARCH,
		GoVersion:     runtime.Version(),
		PluginVersion: tunnel.PluginVersion,
		AppDir:        a.dir,
		DataDir:       a.dataDir,
		LogDir:        filepath.Join(a.dataDir, logDirName),
	}
	var config struct {
		Info struct {
			ProductName    string `json:"productName"`
			ProductVersion string `json:"productVersion"`
			Copyright      string `json:"copyright"`
			Comments       string `json:"comments"`
		} `json:"info"`
	}
	if json.Unmarshal(wailsConfig, &config) == nil {
		if config.Info.ProductName != "" {
			info.Name = config.Info.ProductName
		}
		info.Version = config.Info.ProductVersion
		info.Copyright = config.Info.Copyright
		info.Description = config.Info.Comments
	}
	if a.ctx != nil {
		info.BuildType = wruntime.Environment(a.ctx).BuildType
	}
	if build, ok := debug.ReadBuildInfo(); ok {
		if strings.HasPrefix(build.Main.Path, "github.com/") {
			info.Repository = "https://" + build.Main.Path
		}
		for _, dep := range build.Deps {
			if dep.Path == "github.com/wailsapp/wails/v2" {
				info.WailsVersion = dep.Version
			}
		}
		for _, s := range build.Settings {
			switch s.Key {
			case "vcs.revision":
				info.Commit = s.Value
			case "vcs.time":
				info.CommitTime = s.Value
			case "vcs.modified":
				info.Modified = s.Value == "true"
			}
		}
	}
	return info
}

// OpenURL opens a link of the About screen in the default browser.
func (a *App) OpenURL(url string) {
	if strings.HasPrefix(url, "https://") {
		wruntime.BrowserOpenURL(a.ctx, url)
	}
}
