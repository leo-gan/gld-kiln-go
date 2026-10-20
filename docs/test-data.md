# Test data

`testdata/kiln-v2-cases.json` holds the 46 accept and reject cases for Kiln
wire v2. Each accept case is decoded, compared with the recorded value, and
encoded back to the same bytes. Each reject case must return the recorded
error name: `FRAME`, `LIMIT`, `EOF`, `TYPE`, `UTF8`, `TRAILING`, or `SCHEMA`.

`testdata/reference/kiln_ref.py` is a test-only oracle for Frozen bytes.
Tests shell out to it for Point, Message, lists, optionals, dicts, framed
values, negative zero, and a quiet NaN. The runtime does not import it.

`testdata/schema/point.json` and `testdata/schema/message.json` are the JSON
Schema documents `gld-kilngen-go` compiles into `tests/generated`.
`scripts/check-generated.sh` regenerates those files and diffs them.

`bash scripts/run-tests.sh` runs `go test ./...`.
