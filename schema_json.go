package kiln

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// SchemaFromJSON reads the Kiln JSON Schema subset: booleans, integers,
// floats, strings, binary strings, arrays, objects, $ref of #/$defs/Name,
// required properties, and additionalProperties as dict<str, V>.
func SchemaFromJSON(data []byte) (*Schema, error) {
	keys, fields, err := objectFields(data)
	if err != nil {
		return nil, err
	}
	defs := map[string]json.RawMessage{}
	if raw, ok := fields["$defs"]; ok {
		_, defs, err = objectFields(raw)
		if err != nil {
			return nil, err
		}
	}
	_ = keys
	return compileJSON(data, defs, map[string]bool{})
}

func compileJSON(raw json.RawMessage, defs map[string]json.RawMessage, stack map[string]bool) (*Schema, error) {
	_, fields, err := objectFields(raw)
	if err != nil {
		return nil, err
	}
	if ref, ok := fields["$ref"]; ok {
		name, err := refName(ref)
		if err != nil {
			return nil, err
		}
		body, ok := defs[name]
		if !ok {
			return nil, fmt.Errorf("kiln: unknown $ref %s", name)
		}
		if stack[name] {
			return nil, fmt.Errorf("kiln: cyclic $ref %s", name)
		}
		stack[name] = true
		defer delete(stack, name)
		return compileJSON(body, defs, stack)
	}
	typ, err := jsonString(fields["type"])
	if err != nil {
		return nil, err
	}
	switch typ {
	case "boolean":
		return Prim(KindBool), nil
	case "integer":
		format, _ := jsonString(fields["format"])
		k, err := intKind(format)
		if err != nil {
			return nil, err
		}
		return Prim(k), nil
	case "number":
		format, _ := jsonString(fields["format"])
		if format == "" || format == "double" {
			return Prim(KindF64), nil
		}
		if format == "float" {
			return Prim(KindF32), nil
		}
		return nil, fmt.Errorf("kiln: number format %s", format)
	case "string":
		format, _ := jsonString(fields["format"])
		if format == "binary" {
			return Prim(KindBytes), nil
		}
		if format != "" {
			return nil, fmt.Errorf("kiln: string format %s", format)
		}
		return Prim(KindStr), nil
	case "array":
		items, ok := fields["items"]
		if !ok {
			return nil, fmt.Errorf("kiln: array needs items")
		}
		elem, err := compileJSON(items, defs, stack)
		if err != nil {
			return nil, err
		}
		return ListOf(elem), nil
	case "object":
		if _, ok := fields["properties"]; !ok {
			add, ok := fields["additionalProperties"]
			if !ok {
				return StructOf(), nil
			}
			if isJSONBool(add) {
				return nil, fmt.Errorf("kiln: additionalProperties bool is not a value schema")
			}
			elem, err := compileJSON(add, defs, stack)
			if err != nil {
				return nil, err
			}
			return DictOf(elem), nil
		}
		propKeys, props, err := objectFields(fields["properties"])
		if err != nil {
			return nil, err
		}
		required, all, err := requiredSet(fields["required"])
		if err != nil {
			return nil, err
		}
		out := make([]Field, 0, len(propKeys))
		for _, name := range propKeys {
			fs, err := compileJSON(props[name], defs, stack)
			if err != nil {
				return nil, err
			}
			if !all && !required[name] {
				fs = OptOf(fs)
			}
			out = append(out, Field{Name: name, Type: fs})
		}
		return StructOf(out...), nil
	default:
		return nil, fmt.Errorf("kiln: unsupported schema type %q", typ)
	}
}

func intKind(format string) (Kind, error) {
	switch format {
	case "", "int64":
		if format == "" {
			return KindInt, nil
		}
		return KindI64, nil
	case "int8":
		return KindI8, nil
	case "int16":
		return KindI16, nil
	case "int32":
		return KindI32, nil
	case "uint8":
		return KindU8, nil
	case "uint16":
		return KindU16, nil
	case "uint32":
		return KindU32, nil
	case "uint64":
		return KindU64, nil
	default:
		return 0, fmt.Errorf("kiln: integer format %s", format)
	}
}

func requiredSet(raw json.RawMessage) (map[string]bool, bool, error) {
	if len(raw) == 0 {
		return nil, true, nil
	}
	var names []string
	if err := json.Unmarshal(raw, &names); err != nil {
		return nil, false, err
	}
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set, false, nil
}

func refName(raw json.RawMessage) (string, error) {
	s, err := jsonString(raw)
	if err != nil {
		return "", err
	}
	const prefix = "#/$defs/"
	if !strings.HasPrefix(s, prefix) || strings.Contains(s[len(prefix):], "/") || s[len(prefix):] == "" {
		return "", fmt.Errorf("kiln: $ref %s", s)
	}
	return s[len(prefix):], nil
}

func jsonString(raw json.RawMessage) (string, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return "", fmt.Errorf("kiln: missing string")
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", err
	}
	return s, nil
}

func isJSONBool(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return s == "true" || s == "false"
}

func objectFields(raw json.RawMessage) ([]string, map[string]json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil, nil, err
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return nil, nil, fmt.Errorf("kiln: schema value is not an object")
	}
	keys := make([]string, 0)
	fields := make(map[string]json.RawMessage)
	for dec.More() {
		tok, err = dec.Token()
		if err != nil {
			return nil, nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, nil, fmt.Errorf("kiln: object key")
		}
		var body json.RawMessage
		if err := dec.Decode(&body); err != nil {
			return nil, nil, err
		}
		keys = append(keys, key)
		fields[key] = body
	}
	return keys, fields, nil
}
