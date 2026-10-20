package generated

import (
	"bytes"
	"math"
	"os"
	"testing"

	"github.com/leo-gan/gld-kiln-go"
)

func TestPointRoundTrip(t *testing.T) {
	label := "z"
	in := Point{X: 1, Y: -4, Label: &label}
	buf, err := in.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if in.EncodedSize() != len(buf) {
		t.Fatalf("size %d want %d", in.EncodedSize(), len(buf))
	}
	out, err := DecodePoint(buf)
	if err != nil {
		t.Fatal(err)
	}
	if out.X != 1 || out.Y != -4 || out.Label == nil || *out.Label != "z" {
		t.Fatalf("%+v", out)
	}
	raw, err := os.ReadFile("../../testdata/schema/point.json")
	if err != nil {
		t.Fatal(err)
	}
	schema, err := kiln.SchemaFromJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	dyn, err := kiln.Encode(schema, map[string]any{"x": int32(1), "y": int32(-4), "label": "z"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf, dyn) {
		t.Fatalf("dynamic %x\nstruct %x", dyn, buf)
	}
	absent := Point{X: 8, Y: 9}
	buf, err = absent.Encode()
	if err != nil {
		t.Fatal(err)
	}
	dyn, err = kiln.Encode(schema, map[string]any{"x": int32(8), "y": int32(9)})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf, dyn) {
		t.Fatalf("absent %x\nstruct %x", dyn, buf)
	}
}

func TestMessageRoundTrip(t *testing.T) {
	in := Message{
		FBool: true, FInt32: -5, FInt64: 99, FFloat64: math.Copysign(0, -1),
		FString: "kiln", FBool2: false, FInt322: 8, FString2: "",
		Meta: Meta{Region: "us", Flag: true},
	}
	buf, err := in.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if in.EncodedSize() != len(buf) {
		t.Fatalf("size %d want %d", in.EncodedSize(), len(buf))
	}
	out, err := DecodeMessage(buf)
	if err != nil {
		t.Fatal(err)
	}
	if out.FString != "kiln" || out.Meta.Region != "us" || out.Meta.Flag != true || out.FInt322 != 8 {
		t.Fatalf("%+v", out)
	}
	if math.Float64bits(out.FFloat64) != math.Float64bits(math.Copysign(0, -1)) {
		t.Fatal("lost negative zero")
	}
	raw, err := os.ReadFile("../../testdata/schema/message.json")
	if err != nil {
		t.Fatal(err)
	}
	schema, err := kiln.SchemaFromJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	dyn, err := kiln.Encode(schema, map[string]any{
		"f_bool": true, "f_int32": int32(-5), "f_int64": int64(99),
		"f_float64": math.Copysign(0, -1), "f_string": "kiln",
		"f_bool_2": false, "f_int32_2": int32(8), "f_string_2": "",
		"meta": map[string]any{"region": "us", "flag": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf, dyn) {
		t.Fatalf("dynamic %x\nstruct %x", dyn, buf)
	}
}
