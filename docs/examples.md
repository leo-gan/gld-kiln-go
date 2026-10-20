# Examples

A two-field struct is a schema value. `Encode` accepts a map. A scalar struct
decodes to `*kiln.Record`.

```go
schema, err := kiln.Parse("struct{x:i32,y:i32}")
buf, err := kiln.Encode(schema, map[string]any{
    "x": int32(3),
    "y": int32(-4),
})
got, err := kiln.Decode(schema, buf, kiln.Options{})
rec := got.(*kiln.Record)
x, _ := rec.Get("x")
```

An optional string is nil when it is absent:

```go
schema, err := kiln.SchemaFromJSON(pointJSON)
buf, err := kiln.Encode(schema, map[string]any{
    "x": int32(1),
    "y": int32(2),
})
```

A tensor directory borrows its payload:

```go
block, err := kiln.EncodeTensors([]kiln.Tensor{{
    DType: 2,
    Shape: []uint64{1024},
    Data:  f32Bytes,
}})
got, err := kiln.Decode(kiln.Prim(kiln.KindTensors), block, kiln.Options{})
dir := got.(*kiln.Directory)
_ = dir.Tensors[0].Data // slice of block
```

Generated code is the same layout with typed fields:

```go
label := "z"
buf, err := Point{X: 1, Y: 2, Label: &label}.Encode()
back, err := DecodePoint(buf)
```

`examples/main.go` in this repository encodes the two-integer struct.
