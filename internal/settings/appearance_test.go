package settings

import (
	"reflect"
	"testing"

	"github.com/felipeolliveira/ssm-helper/internal/jsonfile"
	"github.com/felipeolliveira/ssm-helper/internal/msg"
)

// parse reads the appearance object of a JSON/YAML document, like Load does.
func parse(t *testing.T, data string) (Appearance, []msg.Message) {
	t.Helper()
	root, err := jsonfile.Parse([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	return ParseAppearance(jsonfile.Lookup(root, "appearance"))
}

func TestAppearanceDefaultsWhenMissing(t *testing.T) {
	a, problems := parse(t, `{}`)
	if len(problems) > 0 {
		t.Errorf("unexpected problems: %v", problems)
	}
	if !reflect.DeepEqual(a, DefaultAppearance()) {
		t.Errorf("appearance = %+v", a)
	}
	if active := a.ActivePalette(); active.Name != PaletteSSMDark || !active.BuiltIn {
		t.Errorf("active = %+v", active)
	}
}

func TestAppearanceParse(t *testing.T) {
	data := `
appearance:
  PALETTE: "teal"
  palettes:
    teal:
      bg-deep: "#07100f"
      primary: "#2dd4bf"
    paper:
      bg-deep: "#fff"
`
	a, problems := parse(t, data)
	if len(problems) > 0 {
		t.Errorf("unexpected problems: %v", problems)
	}
	teal := a.ActivePalette()
	if teal.Name != "teal" || teal.BuiltIn || teal.Colors["primary"] != "#2dd4bf" || len(teal.Colors) != len(ColorKeys) {
		t.Fatalf("teal = %+v", teal)
	}
	// Missing colors come from the built-in palette with the same kind of background
	if teal.Colors["bgCard"] != builtinColors[PaletteSSMDark]["bgCard"] {
		t.Errorf("dark palette filled from %q", teal.Colors["bgCard"])
	}
	if paper := a.Palettes[1]; paper.Colors["bgCard"] != builtinColors[PaletteSSMLight]["bgCard"] {
		t.Errorf("light palette filled from %q", paper.Colors["bgCard"])
	}
}

func TestAppearanceProblems(t *testing.T) {
	data := `
appearance:
  PALETTE: "nope"
  palettes:
    ssm-dark:
      primary: "#000"
    custom:
      primary: "teal"
      MODE: "dark"
`
	_, problems := parse(t, data)
	want := []msg.Message{
		msg.New(msg.PaletteReservedName, "path", "appearance.palettes.ssm-dark"),
		msg.New(msg.PaletteInvalidColor, "path", "appearance.palettes.custom.primary"),
		msg.New(msg.PaletteUnknownColor, "path", "appearance.palettes.custom.MODE"),
		msg.New(msg.PaletteNotFound, "palette", "nope"),
	}
	if got := problems; !reflect.DeepEqual(got, want) {
		t.Errorf("problems =\n%+v\nwant\n%+v", got, want)
	}
}

func TestAppearanceBuiltinActive(t *testing.T) {
	a, problems := parse(t, `{"appearance": {"palette": "ssm-light", "palettes": {}}}`)
	if len(problems) > 0 {
		t.Errorf("unexpected problems: %v", problems)
	}
	if a.ActivePalette().Name != PaletteSSMLight {
		t.Errorf("active = %q", a.ActivePalette().Name)
	}
}

func TestIsLight(t *testing.T) {
	for hex, want := range map[string]bool{"#fff": true, "#f6f7f9": true, "#07100f": false, "#14171c": false, "bad": false} {
		if IsLight(hex) != want {
			t.Errorf("IsLight(%q) = %v", hex, !want)
		}
	}
}
