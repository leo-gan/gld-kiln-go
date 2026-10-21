package main

import (
	"strings"
	"testing"
)

func TestGenerateListF32UsesBulkHelpers(t *testing.T) {
	raw := []byte(`{
	  "title": "Row",
	  "type": "object",
	  "properties": {
	    "values": {"type": "array", "items": {"type": "number", "format": "float"}}
	  },
	  "required": ["values"]
	}`)
	file, name, err := generate(raw)
	if err != nil {
		t.Fatal(err)
	}
	if name != "Row" {
		t.Fatalf("name %s", name)
	}
	src := string(file)
	if !strings.Contains(src, "kiln.AppendF32s(dst, v.Values)") {
		t.Fatalf("encode did not call AppendF32s:\n%s", src)
	}
	if !strings.Contains(src, "kiln.TakeF32s(buf, n)") {
		t.Fatalf("decode did not call TakeF32s:\n%s", src)
	}
	if strings.Contains(src, "for _, item := range v.Values") {
		t.Fatalf("list still encoded per element:\n%s", src)
	}
	if strings.Contains(src, "kiln.PutF32") {
		t.Fatalf("list still uses PutF32:\n%s", src)
	}
}
