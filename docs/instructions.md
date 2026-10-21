# Instructions

Go 1.22 or newer. The module path is `github.com/leo-gan/gld-kiln-go`.

```bash
go get github.com/leo-gan/gld-kiln-go@v0.2.0
```

```go
import "github.com/leo-gan/gld-kiln-go"
```

Build the generator from this repository:

```bash
go build -o gld-kilngen-go ./cmd/gld-kilngen-go
./gld-kilngen-go --schema testdata/schema/point.json --out ./internal/gen
```

The generator reads a JSON Schema subset: booleans, integers (`int8` through
`int64`, `uint8` through `uint64`), numbers (`float` or `double`), strings,
binary strings, arrays, objects, and `$ref` of `#/$defs/Name`. A property
listed in `required` is present. A property left out of `required` is optional.
If `required` is omitted, every property is present. An object whose only
value schema is `additionalProperties` is `dict<str, V>`. A `tensors` field is
not emitted from that subset.

Run the tests from a checkout:

```bash
bash scripts/run-tests.sh
bash scripts/check-generated.sh
```

`Decode` rejects trailing bytes. `DecodeOne` returns the unread suffix so a
batch of values can share one buffer. `Options.Cap` raises the default count
and string cap of 16777216. `Options.Trusted` skips UTF-8 and quant-shape
checks. Lengths that pass the buffer are still rejected. `Options.Scratch`
reuses caller slices for numeric lists. `Options.Pool` reuses storage across
decodes; a nil pool is the package pool. A pooled slice is stable only until
the next decode on that pool that materializes another pooled list.
