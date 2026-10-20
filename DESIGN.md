# gld-kiln-go design

gld-kiln-go is a from-scratch Kiln wire v2 library for Go 1.22. The runtime
never calls a Kiln library written in another language. The copied Python file
under `testdata/reference/` may be used as a test oracle. It is not a build or
run dependency.

## Goals

Frozen values are positional and little-endian. A struct is the schema: fields
are written in declaration order, with no names and no padding. Dict keys are
sorted by raw UTF-8 bytes. `list<u8>` and `list<i8>` are raw bytes. Other lists
are a count plus concatenated elements. `opt` is a 0/1 tag. A `tensors` field
is one version-2 directory block, and the next field starts at the end of that
block.

The tensor directory keeps numeric payload bytes as the array. Decode returns
those bytes as a slice of the input buffer. A count is checked against the
unread bytes before the decoder allocates element storage. The default cap is
16,777,216 counts and 16 MiB for one string or byte run. A caller can raise it.
A trusted reader skips UTF-8 and quant-shape checks and still rejects a length
past the buffer.

A frame is `KLN0`, four zero bytes, and the FNV-1a 64 schema hash. A hash
mismatch is `SCHEMA` and stops before the first field.

## API

`Encode`, `Decode`, and `DecodeOne` walk a `Schema`. Small scalar structs
decode to `*Record` so the numeric fields are not boxed. Other structs decode
to `map[string]any`. `EncodeTensors` writes a canonical directory.
`SchemaFromJSON` reads the JSON Schema subset used by dynamic values. It does
not invent a `tensors` field from that subset.

`gld-kilngen-go` emits Go structs with `Encode`, `DecodeX`, and `EncodedSize`.
Generated methods call the same primitive put and take helpers as a
hand-written struct.

## Errors

Wire failures use `*Error`. The code is `FRAME`, `LIMIT`, `EOF`, `TYPE`,
`UTF8`, `TRAILING`, or `SCHEMA`.
