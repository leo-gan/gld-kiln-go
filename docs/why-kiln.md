# Why Kiln

Kiln in Go keeps the schema in the struct. Fields are positional. The bytes
carry no field names, no tags, and no padding between Frozen fields. A tensor
directory keeps the numeric bytes as the array instead of wrapping each element.
A hostile count fails before the decoder allocates element storage.

`gld-kilngen-go` emits those structs. Each generated type has `Encode`,
`Decode`, and `EncodedSize`. The generated code calls this library. It does
not link another Kiln implementation.

This library is not the specification.

## The wire

The normative description of the bytes is the Kiln specification:

<https://leo-gan.github.io/gld-kiln/spec/>

Go reads and writes that format. A frame hash names the schema the program
already has. The directory stores dtype, shape, and a view of the payload.
Scales stay bits. The reader does not dequantize them.
