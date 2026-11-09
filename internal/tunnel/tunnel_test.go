package tunnel

import (
	"io"
	"os"
	"strings"
	"testing"

	pluginlog "github.com/aws/session-manager-plugin/src/log"
)

// The plugin's warnings and errors must reach its output, which the app logs.
func TestPluginLogGoesToOutput(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = stdout }()

	setupPluginLog()
	// The plugin gets its logger this way
	logger := pluginlog.Logger(true, "session-manager-plugin")
	logger.Info("hidden info")
	logger.Errorf("Trying to reconnect the session: %s", "wss://example")
	logger.Flush()
	w.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "[plugin ERROR] Trying to reconnect the session: wss://example") {
		t.Fatalf("error line missing from output: %q", got)
	}
	if strings.Contains(got, "hidden info") {
		t.Fatalf("info line should be filtered: %q", got)
	}
}

func TestPluginNoise(t *testing.T) {
	for line, want := range map[string]bool{
		"[plugin ERROR] error getting console mode: The handle is invalid.":      true,
		"[plugin ERROR] error setting console mode: O identificador é inválido.": true,
		"[plugin ERROR] Trying to reconnect the session":                         false,
		"Waiting for connections...":                                             false,
	} {
		if got := isPluginNoise(line); got != want {
			t.Errorf("isPluginNoise(%q) = %v, want %v", line, got, want)
		}
	}
}
