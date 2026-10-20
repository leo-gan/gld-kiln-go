package kiln

import (
	"testing"
)

func BenchmarkStructBatchEncode(b *testing.B) {
	schema, err := Parse("struct{x:i32,y:i32,z:i32,w:i32}")
	if err != nil {
		b.Fatal(err)
	}
	value := map[string]any{"x": int32(1), "y": int32(-2), "z": int32(3), "w": int32(4)}
	const batch = 256
	b.ReportAllocs()
	b.SetBytes(16 * batch)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst := make([]byte, 0, 16*batch)
		for j := 0; j < batch; j++ {
			dst, err = Append(dst, schema, value)
			if err != nil {
				b.Fatal(err)
			}
		}
		sink = dst
	}
}

func BenchmarkStructBatchDecode(b *testing.B) {
	schema, err := Parse("struct{x:i32,y:i32,z:i32,w:i32}")
	if err != nil {
		b.Fatal(err)
	}
	value := map[string]any{"x": int32(1), "y": int32(-2), "z": int32(3), "w": int32(4)}
	const batch = 256
	dst := make([]byte, 0, 16*batch)
	for j := 0; j < batch; j++ {
		dst, err = Append(dst, schema, value)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(dst)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := dst
		for len(buf) > 0 {
			var v any
			v, buf, err = DecodeOne(schema, buf, Options{})
			if err != nil {
				b.Fatal(err)
			}
			sinkV = v
		}
	}
}

func BenchmarkTensor1024Encode(b *testing.B) {
	data := make([]byte, 1024*4)
	for i := 0; i < 1024; i++ {
		data[i*4] = byte(i)
	}
	ts := []Tensor{{DType: 2, Shape: []uint64{1024}, Data: data}}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out, err := EncodeTensors(ts)
		if err != nil {
			b.Fatal(err)
		}
		sink = out
	}
}

func BenchmarkTensor1024Decode(b *testing.B) {
	data := make([]byte, 1024*4)
	ts := []Tensor{{DType: 2, Shape: []uint64{1024}, Data: data}}
	out, err := EncodeTensors(ts)
	if err != nil {
		b.Fatal(err)
	}
	schema := Prim(KindTensors)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v, err := Decode(schema, out, Options{})
		if err != nil {
			b.Fatal(err)
		}
		sinkV = v
	}
}

var (
	sink  []byte
	sinkV any
)
