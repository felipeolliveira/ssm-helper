package jsonfile

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestEncodeKeepsOrderAndRoundTrips(t *testing.T) {
	root := &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
		Key("z"), Strings("b", "2", "a", `quo"te`),
		Key("a"), {Kind: yaml.MappingNode},
	}}
	data, err := Encode(root)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"z\": {\n    \"b\": \"2\",\n    \"a\": \"quo\\\"te\"\n  },\n  \"a\": {}\n}\n"
	if string(data) != want {
		t.Errorf("Encode =\n%s\nwant\n%s", data, want)
	}
	if !json.Valid(data) {
		t.Error("invalid JSON")
	}
	parsed, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if Field(Object(parsed, "z"), "a") != `quo"te` || parsed.Content[0].Value != "z" {
		t.Errorf("round trip lost data or order")
	}
}

func TestParseErrors(t *testing.T) {
	if _, err := Parse([]byte(`{"a": `)); err == nil {
		t.Error("expected error for truncated JSON")
	}
	if _, err := Parse([]byte(`[1, 2]`)); err == nil {
		t.Error("expected error for non-object root")
	}
	if root, err := Parse(nil); err != nil || root.Kind != yaml.MappingNode {
		t.Errorf("empty document: %v", err)
	}
}

func TestLists(t *testing.T) {
	root := &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
		Key("hosts"), StringList([]string{"a.com", "b.com"}),
		Key("empty"), StringList(nil),
	}}
	data, err := Encode(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := "{\n  \"hosts\": [\"a.com\", \"b.com\"],\n  \"empty\": []\n}\n"; string(data) != want {
		t.Errorf("Encode =\n%s", data)
	}
	parsed, _ := Parse(data)
	if got := List(parsed, "hosts"); len(got) != 2 || got[1] != "b.com" {
		t.Errorf("List = %q", got)
	}
	old, _ := Parse([]byte(`{"hosts": "a.com  b.com", "none": null}`))
	if got := List(old, "hosts"); len(got) != 2 {
		t.Errorf("List from string = %q", got)
	}
	if got := List(old, "none"); got == nil || len(got) != 0 {
		t.Errorf("List from null = %q", got)
	}
}

func TestBools(t *testing.T) {
	root := &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
		Key("on"), BoolValue(true),
		Key("off"), BoolValue(false),
	}}
	data, err := Encode(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := "{\n  \"on\": true,\n  \"off\": false\n}\n"; string(data) != want {
		t.Errorf("Encode =\n%s", data)
	}
	parsed, _ := Parse(data)
	if !Bool(parsed, "on", false) || Bool(parsed, "off", true) || !Bool(parsed, "missing", true) {
		t.Error("Bool did not read the values back")
	}
}
