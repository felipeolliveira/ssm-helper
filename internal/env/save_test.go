package env

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSaveRoundTrip(t *testing.T) {
	original, _, err := Load(filepath.Join("..", "..", "env.json.example"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), DefaultFileName)
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	problems, err := Save(path, original)
	if err != nil || len(problems) > 0 {
		t.Fatalf("Save: problems=%v err=%v", problems, err)
	}
	saved, problems, err := Load(path)
	if err != nil || len(problems) > 0 {
		t.Fatalf("Load: problems=%v err=%v", problems, err)
	}
	saved.Path = original.Path
	if !reflect.DeepEqual(saved, original) {
		t.Errorf("round trip changed env:\n%+v\n%+v", saved, original)
	}
	if backup, _ := os.ReadFile(path + ".bak"); string(backup) != "old" {
		t.Errorf("backup = %q", backup)
	}
	// The saved file matches the example, which documents the format
	example, _ := os.ReadFile(filepath.Join("..", "..", "env.json.example"))
	if data, _ := os.ReadFile(path); strings.TrimSpace(string(data)) != strings.TrimSpace(string(example)) {
		t.Errorf("saved file differs from env.json.example:\n%s", data)
	}
}

func TestSaveRejectsInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultFileName)
	e := &Env{Title: "t", Profiles: []Profile{{Name: "p"}}, Targets: []Target{{Name: "t", Profile: "x"}}}
	problems, err := Save(path, e)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) == 0 {
		t.Fatal("expected problems")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file was written: %v", err)
	}
}
