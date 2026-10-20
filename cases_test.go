package kiln

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"strconv"
	"strings"
	"testing"
	"unsafe"
)

func TestV2Cases(t *testing.T) {
	raw, err := os.ReadFile("testdata/kiln-v2-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Cases []struct {
			ID      string          `json:"id"`
			Expect  string          `json:"expect"`
			Input   string          `json:"input"`
			Schema  json.RawMessage `json:"schema"`
			Decoded json.RawMessage `json:"decoded"`
			Error   string          `json:"error"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Cases) != 46 {
		t.Fatalf("want 46 cases, got %d", len(doc.Cases))
	}
	for _, cs := range doc.Cases {
		cs := cs
		t.Run(cs.ID, func(t *testing.T) {
			buf, err := hex.DecodeString(cs.Input)
			if err != nil {
				t.Fatal(err)
			}
			kind, schema, err := schemaFromCase(cs.Schema)
			if err != nil {
				t.Fatal(err)
			}
			var got any
			var hash uint64
			if kind == "frame" {
				hash, got, err = DecodeFramed(schema, buf, Options{})
			} else {
				got, err = Decode(schema, buf, Options{})
			}
			if cs.Expect == "reject" {
				if CodeOf(err) != cs.Error {
					t.Fatalf("error %q, want %s (%v)", CodeOf(err), cs.Error, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if dir, ok := got.(*Directory); ok {
				assertViews(t, buf, dir)
			}
			if m, ok := got.(map[string]any); ok {
				if dir, ok := m["batch"].(*Directory); ok {
					assertViews(t, buf, dir)
				}
			}
			if len(cs.Decoded) > 0 {
				exp := parseJSON(cs.Decoded)
				var projected any
				if kind == "frame" {
					pm := project(got).(map[string]any)
					pm["hash"] = json.Number(strconv.FormatUint(hash, 10))
					projected = pm
				} else {
					projected = project(got)
				}
				if !same(exp, projected) {
					t.Fatalf("decoded\n got %#v\nwant %#v", projected, exp)
				}
			}
			var out []byte
			if kind == "frame" {
				out, err = EncodeFramed(schema, got)
			} else {
				out, err = Encode(schema, got)
			}
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if !bytes.Equal(out, buf) {
				t.Fatalf("re-encode mismatch\n got %x\nwant %x", out, buf)
			}
		})
	}
}

func schemaFromCase(raw json.RawMessage) (string, *Schema, error) {
	raw = bytes.TrimSpace(raw)
	if raw[0] == '"' {
		var name string
		if err := json.Unmarshal(raw, &name); err != nil {
			return "", nil, err
		}
		if name == "tensor-v2" {
			return name, Prim(KindTensors), nil
		}
		if name == "frame" {
			s, err := Parse("struct{id:i64,batch:tensors}")
			return name, s, err
		}
		s, err := Parse(name)
		return name, s, err
	}
	var obj struct {
		Type    string   `json:"type"`
		Element string   `json:"element"`
		Fields  []string `json:"fields"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", nil, err
	}
	switch obj.Type {
	case "list":
		elem, err := Parse(obj.Element)
		if err != nil {
			return "", nil, err
		}
		return "list", ListOf(elem), nil
	case "struct":
		fields := make([]Field, 0, len(obj.Fields))
		for _, f := range obj.Fields {
			name, typ, _ := strings.Cut(f, ":")
			ft, err := Parse(typ)
			if err != nil {
				return "", nil, err
			}
			fields = append(fields, Field{Name: name, Type: ft})
		}
		return "struct", StructOf(fields...), nil
	default:
		return "", nil, Fail("TYPE")
	}
}

func assertViews(t *testing.T, buf []byte, dir *Directory) {
	t.Helper()
	if len(buf) == 0 {
		return
	}
	base := uintptr(unsafe.Pointer(&buf[0]))
	limit := base + uintptr(len(buf))
	for _, tensor := range dir.Tensors {
		if len(tensor.Data) == 0 {
			continue
		}
		p := uintptr(unsafe.Pointer(&tensor.Data[0]))
		if p < base || p+uintptr(len(tensor.Data)) > limit {
			t.Fatalf("tensor payload is not a view of the input")
		}
	}
}

func parseJSON(b []byte) any {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil
	}
	return v
}

func project(v any) any {
	switch x := v.(type) {
	case *Record:
		m := make(map[string]any, len(x.sch.Fields))
		for i, f := range x.sch.Fields {
			m[f.Name] = project(bitsToValue(f.Type.Kind, x.bits[i]))
		}
		return m
	case *Directory:
		return projectDir(x)
	case map[string]any:
		m := make(map[string]any, len(x))
		for k, val := range x {
			m[k] = project(val)
		}
		return m
	case []any:
		a := make([]any, len(x))
		for i, val := range x {
			a[i] = project(val)
		}
		return a
	case []float32:
		a := make([]any, len(x))
		for i, n := range x {
			a[i] = json.Number(strconv.FormatFloat(float64(n), 'g', -1, 32))
		}
		return a
	case []float64:
		a := make([]any, len(x))
		for i, n := range x {
			a[i] = json.Number(strconv.FormatFloat(n, 'g', -1, 64))
		}
		return a
	case []int8:
		a := make([]any, len(x))
		for i, n := range x {
			a[i] = json.Number(strconv.FormatInt(int64(n), 10))
		}
		return a
	case []byte:
		a := make([]any, len(x))
		for i, n := range x {
			a[i] = json.Number(strconv.FormatUint(uint64(n), 10))
		}
		return a
	case int8:
		return json.Number(strconv.FormatInt(int64(x), 10))
	case int16:
		return json.Number(strconv.FormatInt(int64(x), 10))
	case int32:
		return json.Number(strconv.FormatInt(int64(x), 10))
	case int64:
		return json.Number(strconv.FormatInt(x, 10))
	case uint8:
		return json.Number(strconv.FormatUint(uint64(x), 10))
	case uint16:
		return json.Number(strconv.FormatUint(uint64(x), 10))
	case uint32:
		return json.Number(strconv.FormatUint(uint64(x), 10))
	case uint64:
		return json.Number(strconv.FormatUint(x, 10))
	case float32:
		return json.Number(strconv.FormatFloat(float64(x), 'g', -1, 32))
	case float64:
		return json.Number(strconv.FormatFloat(x, 'g', -1, 64))
	default:
		return v
	}
}

func projectDir(d *Directory) map[string]any {
	named := false
	for _, tensor := range d.Tensors {
		if tensor.Name != "" {
			named = true
			break
		}
	}
	flags := []any{"le"}
	if named {
		flags = []any{"names", "le"}
	}
	ts := make([]any, len(d.Tensors))
	for i, tensor := range d.Tensors {
		var name any
		if tensor.Name != "" {
			name = tensor.Name
		}
		var dt any
		if n, ok := dtypeName(tensor.DType); ok {
			dt = n
		} else {
			dt = json.Number(strconv.FormatUint(uint64(tensor.DType), 10))
		}
		shape := make([]any, len(tensor.Shape))
		for j, dim := range tensor.Shape {
			shape[j] = json.Number(strconv.FormatUint(dim, 10))
		}
		ts[i] = map[string]any{
			"name":         name,
			"dtype":        dt,
			"shape":        shape,
			"quant_scheme": json.Number(strconv.FormatUint(uint64(tensor.QuantScheme), 10)),
			"data_hex":     hex.EncodeToString(tensor.Data),
		}
	}
	return map[string]any{
		"version":      json.Number("2"),
		"flags":        flags,
		"tensors":      ts,
		"block_length": json.Number(strconv.Itoa(d.BlockLength)),
	}
}

func same(a, b any) bool {
	switch av := a.(type) {
	case nil:
		return b == nil
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case json.Number:
		bv, ok := b.(json.Number)
		return ok && numEq(av, bv)
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for k, v := range av {
			if !same(v, bv[k]) {
				return false
			}
		}
		return true
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !same(av[i], bv[i]) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func numEq(a, b json.Number) bool {
	if a.String() == b.String() {
		return true
	}
	ar, aok := new(big.Rat).SetString(a.String())
	br, bok := new(big.Rat).SetString(b.String())
	return aok && bok && ar.Cmp(br) == 0
}
