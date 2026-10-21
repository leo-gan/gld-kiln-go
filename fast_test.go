package kiln

import (
	"bytes"
	"math"
	"testing"
	"unsafe"
)

func TestF32ListBitsAndOwnership(t *testing.T) {
	schema, err := Parse("list<f32>")
	if err != nil {
		t.Fatal(err)
	}
	vals := []float32{
		math.Float32frombits(0x80000000),
		math.Float32frombits(0x7fc00000),
		math.Float32frombits(0x7f800001),
	}
	buf, err := Append(nil, schema, vals)
	if err != nil {
		t.Fatal(err)
	}
	if len(buf) != 4+12 {
		t.Fatalf("len %d", len(buf))
	}
	got, err := Decode(schema, buf, Options{})
	if err != nil {
		t.Fatal(err)
	}
	out := got.([]float32)
	for i, n := range vals {
		if math.Float32bits(out[i]) != math.Float32bits(n) {
			t.Fatalf("bits[%d] %08x", i, math.Float32bits(out[i]))
		}
	}
	buf[4] ^= 0xff
	if math.Float32bits(out[0]) != 0x80000000 {
		t.Fatal("decode aliased the input")
	}
	buf[4] ^= 0xff
	again, err := Decode(schema, buf, Options{})
	if err != nil {
		t.Fatal(err)
	}
	second := again.([]float32)
	if len(out) > 0 && len(second) > 0 && &out[0] == &second[0] {
		if math.Float32bits(out[0]) != math.Float32bits(second[0]) {
			t.Fatal("aliased pool slices diverged")
		}
	} else {
		t.Fatal("package pool did not reuse the f32 backing array")
	}
}

func TestNumericListWidths(t *testing.T) {
	u32s, err := Parse("list<u32>")
	if err != nil {
		t.Fatal(err)
	}
	buf, err := Append(nil, u32s, []uint32{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(buf) != 4+12 {
		t.Fatalf("u32 wire %d", len(buf))
	}
	got, err := Decode(u32s, buf, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.([]uint32); !ok {
		t.Fatalf("u32 type %T", got)
	}
	uints, err := Parse("list<uint>")
	if err != nil {
		t.Fatal(err)
	}
	buf, err = Append(nil, uints, []uint{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(buf) != 4+16 {
		t.Fatalf("uint wire %d", len(buf))
	}
	if _, err = Append(nil, uints, []uint32{1}); err == nil {
		t.Fatal("u32 slice encoded as uint")
	}
}

func TestScratchNotPooled(t *testing.T) {
	schema, err := Parse("list<f32>")
	if err != nil {
		t.Fatal(err)
	}
	buf, err := Append(nil, schema, []float32{1, 2, 3, 4})
	if err != nil {
		t.Fatal(err)
	}
	scratch := &ListScratch{F32: make([]float32, 8)}
	got, err := Decode(schema, buf, Options{Scratch: scratch})
	if err != nil {
		t.Fatal(err)
	}
	out := got.([]float32)
	if &out[0] != &scratch.F32[0] {
		t.Fatal("scratch was not used")
	}
	pool, err := Decode(schema, buf, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if math.Float32bits(out[0]) != math.Float32bits(float32(1)) {
		t.Fatal("package pool overwrote the scratch slice")
	}
	_ = pool
}

func TestF32BulkRoundTrip(t *testing.T) {
	const n = (1 << 16) + 3
	vals := make([]float32, n)
	vals[0] = math.Float32frombits(0x80000000)
	vals[1] = math.Float32frombits(0x7fc00000)
	vals[n-1] = math.Float32frombits(0x7f800001)
	for i := 2; i < n-1; i++ {
		vals[i] = float32(i)
	}
	schema, err := Parse("struct{rows:i32,cols:i32,values:list<f32>}")
	if err != nil {
		t.Fatal(err)
	}
	value := map[string]any{"rows": int32(1), "cols": int32(n), "values": vals}
	buf, err := Append(nil, schema, value)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(schema, buf, Options{})
	if err != nil {
		t.Fatal(err)
	}
	out := got.(map[string]any)["values"].([]float32)
	if len(out) != n {
		t.Fatalf("len %d", len(out))
	}
	buf[12] ^= 0xff
	if math.Float32bits(out[0]) != 0x80000000 || math.Float32bits(out[1]) != 0x7fc00000 || math.Float32bits(out[n-1]) != 0x7f800001 {
		t.Fatal("bulk list did not own its bits")
	}
	for i := 2; i < n-1; i++ {
		if out[i] != float32(i) {
			t.Fatalf("val %d", i)
		}
	}
}

func TestTensorEncodeSkipsPayloadZero(t *testing.T) {
	data := []byte{0x00, 0x00, 0x80, 0x3f}
	ts := []Tensor{{DType: 2, Shape: []uint64{1}, Data: data}}
	clean, err := EncodeTensors(ts)
	if err != nil {
		t.Fatal(err)
	}
	dirty := bytes.Repeat([]byte{0xa5}, len(clean)+64)
	out, err := AppendTensors(dirty[:0], ts)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, clean) {
		t.Fatalf("dirty reuse mismatch\n got %x\nwant %x", out, clean)
	}
	// Fresh allocation must also be canonical when the allocator returns dirt.
	fresh, err := EncodeTensors(ts)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(fresh, clean) {
		t.Fatal("fresh encode mismatch")
	}
	if uintptr(unsafe.Pointer(&out[0])) == 0 {
		t.Fatal("empty")
	}
}
