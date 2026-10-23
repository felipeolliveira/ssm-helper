package settings

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/v-felipeolliveira/ssm-helper/internal/env"
	"github.com/v-felipeolliveira/ssm-helper/internal/jsonfile"
)

// Built-in palettes live in the code and cannot be removed; the settings file only adds palettes.
const (
	PaletteSSMDark  = "ssm-dark"
	PaletteSSMLight = "ssm-light"
)

// DefaultPalette is used when the settings do not choose one.
const DefaultPalette = PaletteSSMDark

// ColorKeys are the colors the UI uses, in the order they are written to the
// settings file. The UI turns each one into a CSS variable (bgDeep is --bg-deep).
var ColorKeys = []string{
	"bgDeep",
	"bgBase",
	"bgCard",
	"bgElevated",
	"border",
	"borderStrong",
	"textPrimary",
	"textSecondary",
	"textMuted",
	"textFaint",
	"primary",
	"primary600",
	"primarySoft",
	"primaryDeep",
	"primaryGlow",
	"success",
	"warning",
	"danger",
	"info",
}

var builtinColors = map[string]map[string]string{
	PaletteSSMDark: {
		"bgDeep":        "#14171c",
		"bgBase":        "#1c2027",
		"bgCard":        "#1c2027",
		"bgElevated":    "#232831",
		"border":        "#2e3440",
		"borderStrong":  "#3a4250",
		"textPrimary":   "#e6e9ee",
		"textSecondary": "#c4cad4",
		"textMuted":     "#98a2b3",
		"textFaint":     "#6b7585",
		"primary":       "#5b8cff",
		"primary600":    "#7aa2ff",
		"primarySoft":   "#1f2b45",
		"primaryDeep":   "#0d1b3d",
		"primaryGlow":   "#5b8cff2e",
		"success":       "#4ade80",
		"warning":       "#fbbf24",
		"danger":        "#f87171",
		"info":          "#60a5fa",
	},
	PaletteSSMLight: {
		"bgDeep":        "#f6f7f9",
		"bgBase":        "#ffffff",
		"bgCard":        "#ffffff",
		"bgElevated":    "#f1f3f6",
		"border":        "#e2e5ea",
		"borderStrong":  "#d0d5dd",
		"textPrimary":   "#1b1f24",
		"textSecondary": "#2b3440",
		"textMuted":     "#5f6b7a",
		"textFaint":     "#9aa4b2",
		"primary":       "#2563eb",
		"primary600":    "#1d4fd8",
		"primarySoft":   "#e8efff",
		"primaryDeep":   "#0b2a6b",
		"primaryGlow":   "#2563eb2e",
		"success":       "#15803d",
		"warning":       "#b45309",
		"danger":        "#c62828",
		"info":          "#1d4ed8",
	},
}

var hexColorPattern = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)

type Palette struct {
	Name    string            `json:"name"`
	Colors  map[string]string `json:"colors"`
	BuiltIn bool              `json:"builtIn"`
}

type Appearance struct {
	// Palette is the name of the active palette, built-in or from Palettes.
	Palette string `json:"palette"`
	// Palettes are the added palettes; built-ins are not listed.
	Palettes []Palette `json:"palettes"`
}

// BuiltinPalettes returns ssm-dark and ssm-light.
func BuiltinPalettes() []Palette {
	return []Palette{builtinPalette(PaletteSSMDark), builtinPalette(PaletteSSMLight)}
}

func builtinPalette(name string) Palette {
	colors := make(map[string]string, len(ColorKeys))
	for _, k := range ColorKeys {
		colors[k] = builtinColors[name][k]
	}
	return Palette{Name: name, Colors: colors, BuiltIn: true}
}

func isBuiltinPalette(name string) bool {
	_, ok := builtinColors[name]
	return ok
}

// DefaultAppearance uses ssm-dark and adds no palettes.
func DefaultAppearance() Appearance {
	return Appearance{Palette: DefaultPalette, Palettes: []Palette{}}
}

// ActivePalette returns the active palette, falling back to the default one.
func (a Appearance) ActivePalette() Palette {
	if isBuiltinPalette(a.Palette) {
		return builtinPalette(a.Palette)
	}
	for _, p := range a.Palettes {
		if p.Name == a.Palette {
			return p
		}
	}
	return builtinPalette(DefaultPalette)
}

// IsLight reports whether a color is light, by its relative luminance.
func IsLight(hex string) bool {
	if !hexColorPattern.MatchString(hex) {
		return false
	}
	digits := hex[1:]
	if len(digits) == 3 {
		digits = string([]byte{digits[0], digits[0], digits[1], digits[1], digits[2], digits[2]})
	}
	channel := func(i int) float64 {
		v, _ := strconv.ParseUint(digits[i:i+2], 16, 8)
		return float64(v) / 255
	}
	return 0.2126*channel(0)+0.7152*channel(2)+0.0722*channel(4) > 0.5
}

// ParseAppearance reads an appearance object. Colors missing from
// a palette come from ssm-light or ssm-dark, depending on its background.
func ParseAppearance(node *yaml.Node) (Appearance, []string) {
	a := DefaultAppearance()
	if node == nil {
		return a, nil
	}
	if node.Kind != yaml.MappingNode {
		return a, []string{"appearance (deve ser um objeto)"}
	}
	var problems []string
	if name := firstField(node, "palette", "PALETTE"); name != "" {
		a.Palette = name
	}

	if palettes := jsonfile.Lookup(node, "palettes"); palettes != nil && palettes.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(palettes.Content); i += 2 {
			name, pnode := palettes.Content[i].Value, palettes.Content[i+1]
			if isBuiltinPalette(name) {
				problems = append(problems, fmt.Sprintf("appearance.palettes.%s (nome reservado para a paleta padrão)", name))
				continue
			}
			if !env.NamePattern.MatchString(name) {
				problems = append(problems, fmt.Sprintf("appearance.palettes.%s (%s)", name, env.InvalidNameHint))
			}
			p := Palette{Name: name, Colors: map[string]string{}}
			if pnode.Kind == yaml.MappingNode {
				for j := 0; j+1 < len(pnode.Content); j += 2 {
					key, value := colorKey(pnode.Content[j].Value), pnode.Content[j+1].Value
					if _, known := builtinColors[PaletteSSMDark][key]; !known {
						problems = append(problems, fmt.Sprintf("appearance.palettes.%s.%s (cor desconhecida)", name, key))
						continue
					}
					if !hexColorPattern.MatchString(value) {
						problems = append(problems, fmt.Sprintf("appearance.palettes.%s.%s (cor inválida: use #rgb, #rrggbb ou #rrggbbaa)", name, key))
					}
					p.Colors[key] = value
				}
			}
			base := PaletteSSMDark
			if IsLight(p.Colors["bgDeep"]) {
				base = PaletteSSMLight
			}
			for _, k := range ColorKeys {
				if _, ok := p.Colors[k]; !ok {
					p.Colors[k] = builtinColors[base][k]
				}
			}
			a.Palettes = append(a.Palettes, p)
		}
	} else if palettes != nil && !(palettes.Kind == yaml.ScalarNode && palettes.Tag == "!!null") {
		problems = append(problems, "appearance.palettes (deve ser um objeto { \"nome\": { cores } })")
	}

	found := isBuiltinPalette(a.Palette)
	for _, p := range a.Palettes {
		found = found || p.Name == a.Palette
	}
	if !found {
		problems = append(problems, fmt.Sprintf("appearance.palette (paleta '%s' não existe)", a.Palette))
	}
	return a, problems
}

func marshalAppearance(a Appearance) *yaml.Node {
	palettes := &yaml.Node{Kind: yaml.MappingNode}
	for _, p := range a.Palettes {
		if p.BuiltIn || isBuiltinPalette(p.Name) {
			continue
		}
		pairs := make([]string, 0, 2*len(ColorKeys))
		for _, k := range ColorKeys {
			// Empty colors are left out and filled from a built-in palette when read
			if p.Colors[k] != "" {
				pairs = append(pairs, k, p.Colors[k])
			}
		}
		palettes.Content = append(palettes.Content, jsonfile.Key(p.Name), jsonfile.Strings(pairs...))
	}
	node := jsonfile.Strings("palette", a.Palette)
	node.Content = append(node.Content, jsonfile.Key("palettes"), palettes)
	return node
}

// colorKey accepts the old kebab-case color names (bg-deep) and returns camelCase (bgDeep).
func colorKey(key string) string {
	parts := strings.Split(key, "-")
	for i := 1; i < len(parts); i++ {
		if parts[i] != "" {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

// firstField reads a key, falling back to its old name.
func firstField(node *yaml.Node, key, oldKey string) string {
	if v := jsonfile.Field(node, key); v != "" {
		return v
	}
	return jsonfile.Field(node, oldKey)
}
