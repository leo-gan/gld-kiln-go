// Command benches times a small struct batch and one 1024-element f32 tensor.
package main

import (
	"fmt"
	"time"

	"github.com/leo-gan/gld-kiln-go"
)

func main() {
	schema, err := kiln.Parse("struct{x:i32,y:i32,z:i32,w:i32}")
	if err != nil {
		panic(err)
	}
	value := map[string]any{"x": int32(1), "y": int32(-2), "z": int32(3), "w": int32(4)}
	const batch = 256
	encoded := make([]byte, 0, 16*batch)
	for i := 0; i < batch; i++ {
		encoded, err = kiln.Append(encoded, schema, value)
		if err != nil {
			panic(err)
		}
	}
	fmt.Printf("struct batch encode: %s\n", per(func() {
		dst := make([]byte, 0, len(encoded))
		for i := 0; i < batch; i++ {
			dst, err = kiln.Append(dst, schema, value)
			if err != nil {
				panic(err)
			}
		}
	}, len(encoded)))
	fmt.Printf("struct batch decode: %s\n", per(func() {
		buf := encoded
		for len(buf) > 0 {
			var v any
			v, buf, err = kiln.DecodeOne(schema, buf, kiln.Options{})
			if err != nil {
				panic(err)
			}
			if v == nil {
				panic("nil")
			}
		}
	}, len(encoded)))
	data := make([]byte, 1024*4)
	ts := []kiln.Tensor{{DType: 2, Shape: []uint64{1024}, Data: data}}
	block, err := kiln.EncodeTensors(ts)
	if err != nil {
		panic(err)
	}
	kind := kiln.Prim(kiln.KindTensors)
	fmt.Printf("tensor f32x1024 encode: %s\n", per(func() {
		if _, err = kiln.EncodeTensors(ts); err != nil {
			panic(err)
		}
	}, len(data)))
	fmt.Printf("tensor f32x1024 decode: %s\n", per(func() {
		if _, err = kiln.Decode(kind, block, kiln.Options{}); err != nil {
			panic(err)
		}
	}, len(data)))
}

func per(fn func(), nbytes int) string {
	const rounds = 2000
	fn()
	start := time.Now()
	for i := 0; i < rounds; i++ {
		fn()
	}
	elapsed := time.Since(start)
	ns := float64(elapsed.Nanoseconds()) / float64(rounds)
	mb := float64(nbytes) / ns * 1e3
	return fmt.Sprintf("%.1f ns/op  %.1f MB/s", ns, mb)
}
