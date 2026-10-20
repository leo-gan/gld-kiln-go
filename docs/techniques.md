# Techniques

## Views

`Tensor.Data` is a slice of the input buffer. The caller keeps that buffer
alive for as long as the view is used. An owned copy is an ordinary `copy` into
a new slice after the block has been checked.

## Reuse

`Append` and `AppendTensors` write into a caller buffer. Reset the length to
zero and pass the same slice for the next value. `PutI32`, `PutF32`, and the
other put helpers grow that buffer only when the spare capacity is too small.

## Bulk numeric payloads

On a little-endian host, a `list<f32>` or `list<f64>` is copied with one
`copy` of the wire bytes. A tensor payload is already those bytes. The encoder
copies the slice once into the directory. It does not visit each element.

## Allocation

Before a list, dict, or tensor count allocates, the decoder multiplies the
count by the minimum element size and compares it with the unread bytes.
Overflow is `LIMIT`. A product that does not fit is `EOF`. The list storage
is allocated only after that check succeeds. `CheckCount` is the same test
for generated loops.

## Scalar structs

A struct of at most eight scalars decodes to `*Record`. The bits live in the
record, not in a map slot per field. Larger structs and structs that contain
strings, lists, or tensors still use `map[string]any`.

## Frame

`EncodeFramed` writes `KLN0`, four zero bytes, and `TypeHash`. `DecodeFramed`
compares the hash before it reads the value. A mismatch is `SCHEMA`.
