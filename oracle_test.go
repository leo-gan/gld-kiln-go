package kiln

import (
	"bytes"
	"encoding/hex"
	"math"
	"os/exec"
	"testing"
)

func TestRefOracle(t *testing.T) {
	script := `
import struct, sys
sys.path.insert(0, "testdata/reference")
import kiln_ref
def emit(label, blob):
    print(label)
    print(blob.hex())
emit("point", kiln_ref.encode(kiln_ref.POINT, {"x": 3, "y": -4}))
emit("point-frame", kiln_ref.encode_framed(kiln_ref.POINT, {"x": 3, "y": -4}))
emit("message", kiln_ref.encode(kiln_ref.MESSAGE, {
    "f_bool": True, "f_int32": -5, "f_int64": 99, "f_float64": -0.0,
    "f_string": "kiln", "f_bool_2": False, "f_int32_2": 8, "f_string_2": "",
}))
emit("message-frame", kiln_ref.encode_framed(kiln_ref.MESSAGE, {
    "f_bool": True, "f_int32": -5, "f_int64": 99, "f_float64": -0.0,
    "f_string": "kiln", "f_bool_2": False, "f_int32_2": 8, "f_string_2": "",
}))
emit("list", kiln_ref.encode(("list", ("i32",)), [1, -2, 3]))
emit("opt-none", kiln_ref.encode(("opt", ("str",)), None))
emit("opt-some", kiln_ref.encode(("opt", ("str",)), "ab"))
emit("dict", kiln_ref.encode(("dict", ("u8",)), {"b": 2, "a": 1}))
emit("f32-negzero", kiln_ref.encode(("f32",), -0.0))
emit("f64-negzero", kiln_ref.encode(("f64",), -0.0))
nan = struct.unpack("<d", bytes.fromhex("010000000000f87f"))[0]
emit("f64-nan", kiln_ref.encode(("f64",), nan))
emit("f64-nan-frame", kiln_ref.encode_framed(("f64",), nan))
emit("list-u8", kiln_ref.encode(("list", ("u8",)), b"\x00\xff"))
emit("list-i8", kiln_ref.encode(("list", ("i8",)), [1, -1]))
`
	cmd := exec.Command("python3", "-c", script)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			t.Fatalf("oracle: %v\n%s", err, ee.Stderr)
		}
		t.Fatal(err)
	}
	want := map[string][]byte{}
	var label string
	for _, line := range bytes.Split(bytes.TrimSpace(out), []byte("\n")) {
		if label == "" {
			label = string(line)
			continue
		}
		raw, err := hex.DecodeString(string(line))
		if err != nil {
			t.Fatal(err)
		}
		want[label] = raw
		label = ""
	}
	point, err := Parse("struct{x:i32,y:i32}")
	if err != nil {
		t.Fatal(err)
	}
	msg, err := Parse("struct{f_bool:bool,f_int32:i32,f_int64:i64,f_float64:f64,f_string:str,f_bool_2:bool,f_int32_2:i32,f_string_2:str}")
	if err != nil {
		t.Fatal(err)
	}
	pointV := map[string]any{"x": int32(3), "y": int32(-4)}
	msgV := map[string]any{
		"f_bool": true, "f_int32": int32(-5), "f_int64": int64(99),
		"f_float64": math.Copysign(0, -1), "f_string": "kiln",
		"f_bool_2": false, "f_int32_2": int32(8), "f_string_2": "",
	}
	listS, _ := Parse("list<i32>")
	optS, _ := Parse("opt<str>")
	dictS, _ := Parse("dict<str,u8>")
	f32s, _ := Parse("f32")
	f64s, _ := Parse("f64")
	u8s, _ := Parse("list<u8>")
	i8s, _ := Parse("list<i8>")
	nan := math.Float64frombits(0x7FF8000000000001)
	checks := []struct {
		name   string
		schema *Schema
		value  any
		frame  bool
	}{
		{"point", point, pointV, false},
		{"point-frame", point, pointV, true},
		{"message", msg, msgV, false},
		{"message-frame", msg, msgV, true},
		{"list", listS, []any{int32(1), int32(-2), int32(3)}, false},
		{"opt-none", optS, nil, false},
		{"opt-some", optS, "ab", false},
		{"dict", dictS, map[string]any{"b": uint8(2), "a": uint8(1)}, false},
		{"f32-negzero", f32s, float32(math.Copysign(0, -1)), false},
		{"f64-negzero", f64s, math.Copysign(0, -1), false},
		{"f64-nan", f64s, nan, false},
		{"f64-nan-frame", f64s, nan, true},
		{"list-u8", u8s, []byte{0x00, 0xff}, false},
		{"list-i8", i8s, []int8{1, -1}, false},
	}
	for _, ck := range checks {
		var got []byte
		if ck.frame {
			got, err = EncodeFramed(ck.schema, ck.value)
		} else {
			got, err = Encode(ck.schema, ck.value)
		}
		if err != nil {
			t.Fatalf("%s encode: %v", ck.name, err)
		}
		if !bytes.Equal(got, want[ck.name]) {
			t.Fatalf("%s\n got %x\nwant %x", ck.name, got, want[ck.name])
		}
		if ck.frame {
			if _, _, err = DecodeFramed(ck.schema, got, Options{}); err != nil {
				t.Fatalf("%s framed decode: %v", ck.name, err)
			}
			continue
		}
		back, err := Decode(ck.schema, got, Options{})
		if err != nil {
			t.Fatalf("%s decode: %v", ck.name, err)
		}
		again, err := Encode(ck.schema, back)
		if err != nil {
			t.Fatalf("%s re-encode: %v", ck.name, err)
		}
		if !bytes.Equal(again, got) {
			t.Fatalf("%s round trip\n got %x\nwant %x", ck.name, again, got)
		}
	}
}
