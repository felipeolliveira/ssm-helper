package env

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/v-felipeolliveira/ssm-helper/internal/jsonfile"
	"github.com/v-felipeolliveira/ssm-helper/internal/msg"
)

// Environment files live next to the executable: env.<name>.json, or env.json
// when there is none of those.
const (
	DefaultFileName = "env.json"
	filePrefix      = "env."
	fileSuffix      = ".json"
)

// DefaultName identifies env.json in the list of environments.
const DefaultName = ""

// FileName returns the file of an environment: env.json for DefaultName.
func FileName(name string) string {
	return fileName(filePrefix, fileSuffix, name)
}

func fileName(prefix, suffix, name string) string {
	if name == DefaultName {
		return strings.TrimSuffix(prefix, ".") + suffix
	}
	return prefix + name + suffix
}

// List returns the environment names in dir, sorted. env.json is listed (as
// DefaultName) only when there is no env.<name>.json.
func List(dir string) ([]string, error) {
	names, hasDefault, err := scan(dir, filePrefix, fileSuffix)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 && hasDefault {
		names = []string{DefaultName}
	}
	return names, nil
}

// scan finds <prefix><name><suffix> files and the <prefix-without-dot><suffix> file.
func scan(dir, prefix, suffix string) (names []string, hasDefault bool, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, false, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		file := e.Name()
		if file == fileName(prefix, suffix, DefaultName) {
			hasDefault = true
			continue
		}
		if !strings.HasPrefix(file, prefix) || !strings.HasSuffix(file, suffix) {
			continue
		}
		if name := strings.TrimSuffix(strings.TrimPrefix(file, prefix), suffix); validName(name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, hasDefault, nil
}

func validName(name string) bool {
	return name != DefaultName && NamePattern.MatchString(name) && !strings.Contains(name, ".")
}

// ValidateNewName checks the name of an environment to be created in dir.
func ValidateNewName(dir, name string) error {
	if !validName(name) {
		return msg.Err(msg.EnvNewNameInvalid)
	}
	if _, err := os.Stat(filepath.Join(dir, FileName(name))); err == nil {
		return msg.Err(msg.EnvNameTaken, "name", name)
	}
	return nil
}

// Create writes a new env.<name>.json (an empty environment is accepted, so it
// can be filled in the GUI).
func Create(dir, name string, e *Env) error {
	if err := ValidateNewName(dir, name); err != nil {
		return err
	}
	data, err := Marshal(e)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, FileName(name)), data, 0o600)
}

// Rename renames env.json or env.<name>.json to env.<to>.json.
func Rename(dir, from, to string) error {
	if err := ValidateNewName(dir, to); err != nil {
		return err
	}
	return os.Rename(filepath.Join(dir, FileName(from)), filepath.Join(dir, FileName(to)))
}

// Migration is the result of MigrateLegacy.
type Migration struct {
	// Files are the environment files written, keyed by the old file they came from.
	Files map[string]string
	// AppearanceSources are the YAML files that had an appearance section, in the order they were read.
	AppearanceSources []string
}

// Old file names: config.json/config.<name>.json, and before that config.yaml/config.<name>.yaml
var legacyFormats = [][2]string{{"config.", ".json"}, {"config.", ".yaml"}}

// MigrateLegacy converts the old config files into env files when dir has no
// env file yet. The newest old format found is used, and the old files are
// left untouched. Names are kept: config.qa.yaml becomes env.qa.json.
func MigrateLegacy(dir string) (Migration, error) {
	m := Migration{Files: map[string]string{}}
	if names, hasDefault, err := scan(dir, filePrefix, fileSuffix); err != nil || len(names) > 0 || hasDefault {
		return m, err
	}

	for _, format := range legacyFormats {
		names, hasDefault, err := scan(dir, format[0], format[1])
		if err != nil {
			return m, err
		}
		if hasDefault {
			names = append([]string{DefaultName}, names...)
		}
		if len(names) == 0 {
			continue
		}
		for _, name := range names {
			from := fileName(format[0], format[1], name)
			data, err := os.ReadFile(filepath.Join(dir, from))
			if err != nil {
				return m, err
			}
			e, _, err := Parse(from, data)
			if err != nil {
				return m, msg.Wrap(msg.EnvMigrationFailed, err, "file", from)
			}
			out, err := Marshal(e)
			if err != nil {
				return m, err
			}
			to := FileName(name)
			if err := os.WriteFile(filepath.Join(dir, to), out, 0o600); err != nil {
				return m, err
			}
			m.Files[from] = to
			if AppearanceNode(data) != nil {
				m.AppearanceSources = append(m.AppearanceSources, from)
			}
		}
		return m, nil
	}
	return m, nil
}

// AppearanceNode returns the appearance section of an old YAML config, if any.
func AppearanceNode(data []byte) *yaml.Node {
	root, err := jsonfile.Parse(data)
	if err != nil {
		return nil
	}
	return jsonfile.Object(root, "appearance")
}
