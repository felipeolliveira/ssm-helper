package settings

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, problems, source, err := Load(dir)
	if err != nil || len(problems) > 0 || source != SourceNone || !reflect.DeepEqual(s, Default()) {
		t.Fatalf("missing file: %+v %v %v %v", s, problems, source, err)
	}

	teal := builtinPalette(PaletteSSMDark)
	teal.Name, teal.BuiltIn = "teal", false
	teal.Colors["primary"] = "#2dd4bf"
	s = Settings{ActiveEnv: "qa", Language: "en", HideSensitive: true, OpenChromeOnConnect: false, Appearance: Appearance{Palette: "teal", Palettes: []Palette{teal}}}
	if err := Save(dir, s); err != nil {
		t.Fatal(err)
	}
	loaded, problems, source, err := Load(dir)
	if err != nil || len(problems) > 0 || source != SourceFile {
		t.Fatalf("problems=%v source=%v err=%v", problems, source, err)
	}
	if !reflect.DeepEqual(loaded, s) {
		t.Errorf("round trip:\n%+v\n%+v", loaded, s)
	}
}

func TestLoadLegacyFile(t *testing.T) {
	dir := t.TempDir()
	legacy := `{
  "ACTIVE_CONFIG": "qa",
  "appearance": {"PALETTE": "teal", "palettes": {"teal": {"bg-deep": "#07100f", "primary-600": "#14b8a6"}}}
}`
	if err := os.WriteFile(filepath.Join(dir, LegacyFileName), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	s, problems, source, err := Load(dir)
	if err != nil || len(problems) > 0 || source != SourceLegacyFile {
		t.Fatalf("problems=%v source=%v err=%v", problems, source, err)
	}
	teal := s.Appearance.ActivePalette()
	if s.HideSensitive || !s.OpenChromeOnConnect {
		t.Errorf("legacy file should use the default preferences: %+v", s)
	}
	if s.ActiveEnv != "qa" || teal.Name != "teal" || teal.Colors["bgDeep"] != "#07100f" || teal.Colors["primary600"] != "#14b8a6" {
		t.Errorf("legacy settings = %+v", s)
	}
}

func TestValidateAppearance(t *testing.T) {
	bad := Appearance{Palette: "missing", Palettes: []Palette{{Name: "x", Colors: map[string]string{"primary": "red"}}}}
	problems, err := ValidateAppearance(bad)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 2 {
		t.Errorf("problems = %q", problems)
	}
}

func TestValidLanguage(t *testing.T) {
	if ValidLanguage("en") != "en" || ValidLanguage("pt-BR") != "pt-BR" || ValidLanguage("fr") != "" {
		t.Error("ValidLanguage accepted or rejected the wrong values")
	}
}
