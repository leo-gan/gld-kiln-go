# gld-kiln-go

A from-scratch Kiln wire v2 library for Go. The runtime and the code generator
are Go. They do not wrap, link, or vendor another language's Kiln library.
Python `testdata/reference/kiln_ref.py` is a test oracle only.

The module path is `github.com/leo-gan/gld-kiln-go`. The import name is `kiln`.
The generator command is `gld-kilngen-go`.

Copyright (c) 2026 Leonid Ganeline. MIT.

## Install

```bash
go get github.com/leo-gan/gld-kiln-go@v0.2.0
```

```go
import "github.com/leo-gan/gld-kiln-go"
```

Generate structs from a JSON Schema file:

```bash
gld-kilngen-go --schema point.json --out ./internal/gen
```

## Docs

- [Why Kiln](https://leo-gan.github.io/gld-kiln-go/why-kiln/)
- [Instructions](https://leo-gan.github.io/gld-kiln-go/instructions/)
- [Examples](https://leo-gan.github.io/gld-kiln-go/examples/)
- [Techniques](https://leo-gan.github.io/gld-kiln-go/techniques/)
- [Test data](https://leo-gan.github.io/gld-kiln-go/test-data/)

This library is not the Kiln specification.
