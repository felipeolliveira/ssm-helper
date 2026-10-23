package main

import (
	"embed"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"github.com/v-felipeolliveira/ssm-helper/internal/tunnel"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Each SSM tunnel runs as a child process of this same executable
	if len(os.Args) > 1 && os.Args[1] == tunnel.PluginArg {
		tunnel.RunPlugin(os.Args[1:])
		return
	}

	dir, dataDir, err := defaultPaths()
	if err != nil {
		println("Error:", err.Error())
		os.Exit(1)
	}
	app := NewApp(dir, dataDir)

	err = wails.Run(&options.App{
		Title:     "SSM Helper",
		Width:     1120,
		Height:    780,
		MinWidth:  820,
		MinHeight: 560,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: app.startupBackground(),
		OnStartup:        app.startupHook,
		OnShutdown:       app.shutdown,
		Windows: &windows.Options{
			// Keep the WebView2 data in the app folder instead of %APPDATA%
			WebviewUserDataPath: filepath.Join(dataDir, "webview"),
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "ssm-helper-3f6c1a2e",
		},
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
