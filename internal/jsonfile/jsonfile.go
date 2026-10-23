// Package jsonfile reads and writes the app's JSON files keeping the order of
// object keys, so profiles, targets and palettes stay in the order written.
// Reading goes through the YAML parser, which accepts JSON and keeps the order.
package jsonfile

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// FormatError wraps a parse failure.
type FormatError struct{ Err error }

func (e *FormatError) Error() string { return e.Err.Error() }

// Parse returns the root object of a JSON (or YAML) document; an empty document is an empty object.
func Parse(data []byte) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, &FormatError{Err: err}
	}
	if len(doc.Content) == 0 {
		return &yaml.Node{Kind: yaml.MappingNode}, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, &FormatError{Err: errors.New("o conteúdo principal deve ser um objeto { \"chave\": valor }")}
	}
	return root, nil
}

// Lookup returns the value of key in an object node.
func Lookup(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// Object returns the value of key when it is an object.
func Object(node *yaml.Node, key string) *yaml.Node {
	value := Lookup(node, key)
	if value == nil || value.Kind != yaml.MappingNode {
		return nil
	}
	return value
}

// Field returns a scalar value as text; missing, null and non-scalar values are empty.
func Field(node *yaml.Node, key string) string {
	value := Lookup(node, key)
	if value == nil || value.Kind != yaml.ScalarNode || value.Tag == "!!null" {
		return ""
	}
	return value.Value
}

// List returns a list of strings; a string value is split on spaces (the old format).
func List(node *yaml.Node, key string) []string {
	value := Lookup(node, key)
	if value == nil {
		return []string{}
	}
	switch value.Kind {
	case yaml.SequenceNode:
		items := make([]string, 0, len(value.Content))
		for _, item := range value.Content {
			if item.Kind == yaml.ScalarNode && strings.TrimSpace(item.Value) != "" {
				items = append(items, strings.TrimSpace(item.Value))
			}
		}
		return items
	case yaml.ScalarNode:
		if value.Tag == "!!null" {
			return []string{}
		}
		return strings.Fields(value.Value)
	}
	return []string{}
}

// StringList builds a list of strings.
func StringList(values []string) *yaml.Node {
	node := &yaml.Node{Kind: yaml.SequenceNode}
	for _, v := range values {
		node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v})
	}
	return node
}

// Bool reads a boolean, returning fallback when the key is missing or not a boolean.
func Bool(node *yaml.Node, key string, fallback bool) bool {
	switch strings.ToLower(Field(node, key)) {
	case "true":
		return true
	case "false":
		return false
	}
	return fallback
}

// BoolValue builds a boolean value.
func BoolValue(v bool) *yaml.Node {
	value := "false"
	if v {
		value = "true"
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: value}
}

// Key builds an object key.
func Key(name string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Value: name}
}

// Strings builds an object of string values from key/value pairs.
func Strings(pairs ...string) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode}
	for i := 0; i+1 < len(pairs); i += 2 {
		node.Content = append(node.Content, Key(pairs[i]), &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: strings.TrimSpace(pairs[i+1]),
		})
	}
	return node
}

// Encode writes an object node as indented JSON. Scalars are written as
// strings, except booleans built with BoolValue; lists are written on one line.
func Encode(root *yaml.Node) ([]byte, error) {
	var b bytes.Buffer
	if err := encode(&b, root, ""); err != nil {
		return nil, err
	}
	b.WriteByte('\n')
	return b.Bytes(), nil
}

func encode(b *bytes.Buffer, node *yaml.Node, indent string) error {
	switch node.Kind {
	case yaml.MappingNode:
		if len(node.Content) == 0 {
			b.WriteString("{}")
			return nil
		}
		b.WriteString("{\n")
		inner := indent + "  "
		for i := 0; i+1 < len(node.Content); i += 2 {
			b.WriteString(inner)
			if err := writeString(b, node.Content[i].Value); err != nil {
				return err
			}
			b.WriteString(": ")
			if err := encode(b, node.Content[i+1], inner); err != nil {
				return err
			}
			if i+2 < len(node.Content) {
				b.WriteByte(',')
			}
			b.WriteByte('\n')
		}
		b.WriteString(indent + "}")
	case yaml.SequenceNode:
		if len(node.Content) == 0 {
			b.WriteString("[]")
			return nil
		}
		b.WriteByte('[')
		for i, item := range node.Content {
			if i > 0 {
				b.WriteString(", ")
			}
			if err := encode(b, item, indent); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case yaml.ScalarNode:
		if node.Tag == "!!bool" && (node.Value == "true" || node.Value == "false") {
			b.WriteString(node.Value)
			return nil
		}
		return writeString(b, node.Value)
	default:
		return errors.New("jsonfile: unsupported node")
	}
	return nil
}

func writeString(b *bytes.Buffer, s string) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	b.Write(data)
	return nil
}

// WriteAtomic writes data to path through a temporary file. When backup is
// true, the previous file is kept as <path>.bak.
func WriteAtomic(path string, data []byte, backup bool) error {
	if backup {
		if previous, err := os.ReadFile(path); err == nil {
			if err := os.WriteFile(path+".bak", previous, 0o600); err != nil {
				return err
			}
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
