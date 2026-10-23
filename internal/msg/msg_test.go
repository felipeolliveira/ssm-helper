package msg

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestErrorIsJSON(t *testing.T) {
	err := Wrap(TunnelPortInUse, errors.New("bind: address in use"), "port", "8443")
	var m Message
	if jerr := json.Unmarshal([]byte(err.Error()), &m); jerr != nil {
		t.Fatal(jerr)
	}
	want := Message{Code: TunnelPortInUse, Params: map[string]string{"port": "8443"}, Detail: "bind: address in use"}
	if !reflect.DeepEqual(m, want) {
		t.Errorf("message = %+v", m)
	}
}

func TestIsAndUnwrap(t *testing.T) {
	sentinel := Err(EnvNotFound)
	wrapped := fmt.Errorf("loading: %w", Err(EnvNotFound))
	if !errors.Is(wrapped, sentinel) {
		t.Error("errors.Is should match the same code")
	}
	if errors.Is(Err(SSOLoginExpired), sentinel) {
		t.Error("errors.Is matched another code")
	}
	cause := errors.New("aws said no")
	if !errors.Is(Wrap(SSOLoginFailed, cause), cause) {
		t.Error("Unwrap should reach the cause")
	}
}

func TestFrom(t *testing.T) {
	if m := From(fmt.Errorf("x: %w", Err(ChromeNotFound))); m.Code != ChromeNotFound {
		t.Errorf("From(*Error) = %+v", m)
	}
	if m := From(errors.New("boom")); m.Code != UnexpectedError || m.Detail != "boom" {
		t.Errorf("From(plain error) = %+v", m)
	}
	if m := From(nil); m.Code != "" {
		t.Errorf("From(nil) = %+v", m)
	}
}
