package env

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/felipeolliveira/ssm-helper/internal/msg"
)

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	if names, _ := List(dir); len(names) != 0 {
		t.Errorf("empty dir: %q", names)
	}

	writeFiles(t, dir, map[string]string{DefaultFileName: "{}", "env.json.example": "{}", "notes.json": "{}", "env.json.bak": "{}"})
	if names, _ := List(dir); !reflect.DeepEqual(names, []string{DefaultName}) {
		t.Errorf("only env.json: %q", names)
	}

	// env.json is ignored once there are named environments
	writeFiles(t, dir, map[string]string{"env.qa.json": "{}", "env.prod.json": "{}", "env.a.b.json": "{}"})
	if names, _ := List(dir); !reflect.DeepEqual(names, []string{"prod", "qa"}) {
		t.Errorf("named environments: %q", names)
	}
}

func TestCreateAndRename(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{DefaultFileName: "{}"})

	if err := Create(dir, "qa", &Env{Title: "QA"}); err != nil {
		t.Fatal(err)
	}
	if err := Create(dir, "qa", &Env{}); msg.From(err).Code != msg.EnvNameTaken {
		t.Errorf("duplicate: %v", err)
	}
	for _, bad := range []string{"", "a.b", "a b"} {
		if err := ValidateNewName(dir, bad); err == nil {
			t.Errorf("ValidateNewName(%q) accepted", bad)
		}
	}
	if err := Rename(dir, DefaultName, "dev"); err != nil {
		t.Fatal(err)
	}
	if names, _ := List(dir); !reflect.DeepEqual(names, []string{"dev", "qa"}) {
		t.Errorf("after rename: %q", names)
	}
	e, _, err := Load(filepath.Join(dir, FileName("qa")))
	if err != nil || e.Title != "QA" {
		t.Errorf("created env: %+v %v", e, err)
	}
}

func TestMigrateFromYAML(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"config.yaml":         "variables:\n  MENU_TITLE: \"QA\"\nappearance:\n  PALETTE: \"ssm-light\"\nprofiles: {}\ntargets: {}\n",
		"config.prod.yaml":    "variables:\n  MENU_TITLE: \"Prod\"\n",
		"config.yaml.example": "ignored",
		"config.yaml.bak":     "ignored",
	})

	m, err := MigrateLegacy(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"config.yaml": DefaultFileName, "config.prod.yaml": "env.prod.json"}
	if !reflect.DeepEqual(m.Files, want) || !reflect.DeepEqual(m.AppearanceSources, []string{"config.yaml"}) {
		t.Errorf("migration = %+v", m)
	}
	e, _, err := Load(filepath.Join(dir, "env.prod.json"))
	if err != nil || e.Title != "Prod" {
		t.Errorf("migrated env: %+v %v", e, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); err != nil {
		t.Error("YAML file was removed")
	}

	// Once there is an env file, nothing is migrated again
	m, err = MigrateLegacy(dir)
	if err != nil || len(m.Files) != 0 {
		t.Errorf("second run: %+v %v", m, err)
	}
}

// config.json files (the previous JSON format) win over older YAML files.
func TestMigrateFromOldJSON(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"config.json":    `{"variables": {"MENU_TITLE": "From JSON"}, "targets": {"web": {"DNS_HOSTS": "a.com b.com"}}}`,
		"config.qa.json": `{"variables": {"MENU_TITLE": "QA"}}`,
		"config.yaml":    "variables:\n  MENU_TITLE: \"From YAML\"\n",
	})
	m, err := MigrateLegacy(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"config.json": DefaultFileName, "config.qa.json": "env.qa.json"}; !reflect.DeepEqual(m.Files, want) {
		t.Errorf("files = %v", m.Files)
	}
	e, _, _ := Load(filepath.Join(dir, DefaultFileName))
	web, _ := e.Target("web")
	if e.Title != "From JSON" || !reflect.DeepEqual(web.DNSHosts, []string{"a.com", "b.com"}) {
		t.Errorf("migrated env = %+v", e)
	}
}
