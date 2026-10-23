// Package settings keeps the app preferences that are not part of an
// environment: the selected environment and the appearance. They are stored in
// ssm-helper.json next to the executable, so the app stays portable.
package settings

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/v-felipeolliveira/ssm-helper/internal/jsonfile"
)

const (
	FileName = "ssm-helper.json"
	// LegacyFileName is the previous name, read when FileName does not exist yet.
	LegacyFileName = "ssm-helper.settings.json"
)

type Settings struct {
	// ActiveEnv is the selected environment name; env.DefaultName is env.json.
	ActiveEnv  string     `json:"activeEnv"`
	Appearance Appearance `json:"appearance"`
}

// Default uses env.json and the ssm-dark palette.
func Default() Settings {
	return Settings{Appearance: DefaultAppearance()}
}

// Source tells where Load read the settings from.
type Source int

const (
	SourceNone Source = iota
	SourceFile
	SourceLegacyFile
)

// Load reads FileName (or LegacyFileName) from dir. Missing files give the
// defaults; problems in the file are returned and the affected values fall
// back to the defaults.
func Load(dir string) (Settings, []string, Source, error) {
	s := Default()
	source := SourceFile
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if errors.Is(err, os.ErrNotExist) {
		source = SourceLegacyFile
		data, err = os.ReadFile(filepath.Join(dir, LegacyFileName))
		if errors.Is(err, os.ErrNotExist) {
			return s, nil, SourceNone, nil
		}
	}
	if err != nil {
		return s, nil, source, err
	}
	root, err := jsonfile.Parse(data)
	if err != nil {
		return s, nil, source, err
	}
	s.ActiveEnv = firstField(root, "activeEnv", "ACTIVE_CONFIG")
	appearance, problems := ParseAppearance(jsonfile.Lookup(root, "appearance"))
	if len(problems) == 0 {
		s.Appearance = appearance
	}
	return s, problems, source, nil
}

// ValidateAppearance returns the problems of an appearance edited in the GUI.
func ValidateAppearance(a Appearance) ([]string, error) {
	data, err := jsonfile.Encode(&yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
		jsonfile.Key("appearance"), marshalAppearance(a),
	}})
	if err != nil {
		return nil, err
	}
	root, err := jsonfile.Parse(data)
	if err != nil {
		return nil, err
	}
	_, problems := ParseAppearance(jsonfile.Lookup(root, "appearance"))
	return problems, nil
}

// Save writes the settings to FileName in dir.
func Save(dir string, s Settings) error {
	root := jsonfile.Strings("activeEnv", s.ActiveEnv)
	root.Content = append(root.Content, jsonfile.Key("appearance"), marshalAppearance(s.Appearance))
	data, err := jsonfile.Encode(root)
	if err != nil {
		return err
	}
	return jsonfile.WriteAtomic(filepath.Join(dir, FileName), data, false)
}
