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

On a little-endian host, a numeric list is one copy of the wire bytes.
`list<f32>` is a `[]float32`, `list<i32>` is a `[]int32`, and the other plain
numbers use their own slices. A `u32` is four bytes. It is not written into a
`uint` or `uint64` slot. Negative zero (`0x80000000`) and a quiet NaN
(`0x7fc00000`) keep their bits. `AppendF32s` and `TakeF32s` are that copy.
Generated list fields call them. A tensor payload is copied once into the
directory. The encoder does not zero those payload bytes before the copy.

## Pooled lists

`Decode` with a nil `Options.Pool` uses the package list pool. Set `Pool` to
a `ListPool` to keep that storage private. `Options.Scratch` supplies
caller slices for the first list of each kind when the capacity is large
enough. Scratch slices are not pooled.

A slice drawn from a pool aliases storage that the pool reuses. It is stable
only until the next decode on that pool that materializes another pooled
list. Do not retain it past that decode.

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
