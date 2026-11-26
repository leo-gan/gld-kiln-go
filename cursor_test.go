package kiln

import (
	"bytes"
	"math"
	"testing"
)

func TestCursorRoundtripAndWireIdentity(t *testing.T) {
	sch := StructOf(
		Field{Name: "b", Type: Prim(KindBool)},
		Field{Name: "i32", Type: Prim(KindI32)},
		Field{Name: "i64", Type: Prim(KindI64)},
		Field{Name: "f64", Type: Prim(KindF64)},
		Field{Name: "s", Type: Prim(KindStr)},
		Field{Name: "vals", Type: ListOf(Prim(KindF64))},
	)
	vals := []float64{1.5, math.Copysign(0, -1), 42.25}
	label := "0123456789abcdef-utf8-é"

	w := NewWriter(128)
	w.Reset(128)
	w.PutBool(true)
	w.PutI32(-7)
	w.PutI64(9000000000)
	w.PutF64(-12.5)
	if err := w.PutString(label); err != nil {
		t.Fatal(err)
	}
	if err := w.PutF64s(vals); err != nil {
		t.Fatal(err)
	}
	got := append([]byte(nil), w.Bytes()...)

	want, err := Encode(sch, map[string]any{
		"b":    true,
		"i32":  int32(-7),
		"i64":  int64(9000000000),
		"f64":  -12.5,
		"s":    label,
		"vals": vals,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("wire mismatch:\n got=%x\nwant=%x", got, want)
	}

	r := NewReader(got, Options{})
	b, err := r.Bool()
	if err != nil || !b {
		t.Fatalf("bool: %v %v", b, err)
	}
	i32, err := r.I32()
	if err != nil || i32 != -7 {
		t.Fatalf("i32: %v %v", i32, err)
	}
	i64, err := r.I64()
	if err != nil || i64 != 9000000000 {
		t.Fatalf("i64: %v %v", i64, err)
	}
	f64, err := r.F64()
	if err != nil || f64 != -12.5 {
		t.Fatalf("f64: %v %v", f64, err)
	}
	s, err := r.String()
	if err != nil || s != label {
		t.Fatalf("str: %q %v", s, err)
	}
	back, err := r.F64s()
	if err != nil || len(back) != len(vals) || math.Float64bits(back[1]) != math.Float64bits(vals[1]) {
		t.Fatalf("f64s: %v %v", back, err)
	}
	if err := r.Finish(); err != nil {
		t.Fatalf("finish: %v", err)
	}

	// Mutating source buffer after decode must not corrupt decoded strings.
	for i := range got {
		got[i] = 'X'
	}
	if s != label {
		t.Fatalf("string aliased mutable input buffer: got %q want %q", s, label)
	}

	// Reject bad UTF-8 and trailing bytes.
	badUTF8 := []byte{2, 0, 0, 0, 0xff, 0xfe}
	rb := NewReader(badUTF8, Options{})
	if _, err := rb.String(); err == nil {
		t.Fatal("expected UTF8 error")
	}
	rt := NewReader([]byte{1, 0}, Options{})
	if _, err := rt.Bool(); err != nil {
		t.Fatal(err)
	}
	if err := rt.Finish(); err == nil {
		t.Fatal("expected TRAILING error")
	}
}

func TestUtf8ValidOffsets(t *testing.T) {
	for n := 0; n <= 40; n++ {
		buf := bytes.Repeat([]byte{'a'}, n)
		if !utf8Valid(buf) {
			t.Fatalf("len %d: valid ascii rejected", n)
		}
		for i := 0; i < n; i++ {
			buf[i] = 0xff
			if utf8Valid(buf) {
				t.Fatalf("len %d off %d: invalid byte accepted", n, i)
			}
			buf[i] = 'a'
		}
	}
}
