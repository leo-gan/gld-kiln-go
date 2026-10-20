package main

import (
	"fmt"

	"github.com/leo-gan/gld-kiln-go"
)

func main() {
	schema, err := kiln.Parse("struct{x:i32,y:i32}")
	if err != nil {
		panic(err)
	}
	buf, err := kiln.Encode(schema, map[string]any{"x": int32(3), "y": int32(-4)})
	if err != nil {
		panic(err)
	}
	fmt.Printf("%x\n", buf)
	got, err := kiln.Decode(schema, buf, kiln.Options{})
	if err != nil {
		panic(err)
	}
	rec := got.(*kiln.Record)
	x, _ := rec.Get("x")
	y, _ := rec.Get("y")
	fmt.Printf("x=%v y=%v\n", x, y)
}
