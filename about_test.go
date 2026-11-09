package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/felipeolliveira/ssm-helper/internal/tunnel"
)

func TestAboutReadsWailsJSON(t *testing.T) {
	var config struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	data, err := os.ReadFile("wails.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}

	a := &App{dir: "app", dataDir: "data"}
	info := a.About()
	if info.Version == "" || info.Version != config.Info.ProductVersion {
		t.Errorf("Version = %q, want %q", info.Version, config.Info.ProductVersion)
	}
	if info.PluginVersion != tunnel.PluginVersion || info.GoVersion == "" || info.Name == "" {
		t.Errorf("incomplete info: %+v", info)
	}
}
