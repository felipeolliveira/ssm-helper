package settings

import (
	"reflect"
	"testing"

	"github.com/v-felipeolliveira/ssm-helper/internal/jsonfile"
)

// parse reads the appearance object of a JSON/YAML document, like Load does.
func parse(t *testing.T, data string) (Appearance, []string) {
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
	want := []string{
		"appearance.palettes.ssm-dark (nome reservado para a paleta padrão)",
		"appearance.palettes.custom.primary (cor inválida: use #rgb, #rrggbb ou #rrggbbaa)",
		"appearance.palettes.custom.MODE (cor desconhecida)",
		"appearance.palette (paleta 'nope' não existe)",
	}
	if got := problems; !reflect.DeepEqual(got, want) {
		t.Errorf("problems =\n%q\nwant\n%q", got, want)
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
