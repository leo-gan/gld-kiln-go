package kiln

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	DefaultCap = 16777216
	maxDepth   = 64
	fnvOffset  = uint64(14695981039346656037)
	fnvPrime   = uint64(1099511628211)
)

// Error is a wire failure. Code is FRAME, LIMIT, EOF, TYPE, UTF8, TRAILING, or SCHEMA.
type Error struct{ Code string }

func (e *Error) Error() string { return e.Code }

func Fail(code string) error { return &Error{Code: code} }

func CodeOf(err error) string {
	if e, ok := err.(*Error); ok && e != nil {
		return e.Code
	}
	return ""
}

type Kind uint8

const (
	KindBool Kind = iota + 1
	KindI8
	KindI16
	KindI32
	KindI64
	KindInt
	KindU8
	KindU16
	KindU32
	KindU64
	KindUint
	KindF32
	KindF64
	KindStr
	KindBytes
	KindList
	KindOpt
	KindDict
	KindStruct
	KindTensors
)

type Field struct {
	Name string
	Type *Schema
}

type Schema struct {
	Kind   Kind
	Elem   *Schema
	Fields []Field
	fast   int8 // 1: scalar struct, -1: not
}

func Prim(k Kind) *Schema { return &Schema{Kind: k} }

func ListOf(elem *Schema) *Schema { return &Schema{Kind: KindList, Elem: elem} }
func OptOf(elem *Schema) *Schema  { return &Schema{Kind: KindOpt, Elem: elem} }
func DictOf(elem *Schema) *Schema { return &Schema{Kind: KindDict, Elem: elem} }
func StructOf(fields ...Field) *Schema {
	s := &Schema{Kind: KindStruct, Fields: fields}
	if pureScalar(s) {
		s.fast = 1
	} else {
		s.fast = -1
	}
	return s
}

func Parse(s string) (*Schema, error) {
	switch s {
	case "bool":
		return Prim(KindBool), nil
	case "i8":
		return Prim(KindI8), nil
	case "i16":
		return Prim(KindI16), nil
	case "i32":
		return Prim(KindI32), nil
	case "i64":
		return Prim(KindI64), nil
	case "int":
		return Prim(KindInt), nil
	case "u8":
		return Prim(KindU8), nil
	case "u16":
		return Prim(KindU16), nil
	case "u32":
		return Prim(KindU32), nil
	case "u64":
		return Prim(KindU64), nil
	case "uint":
		return Prim(KindUint), nil
	case "f32":
		return Prim(KindF32), nil
	case "f64":
		return Prim(KindF64), nil
	case "str":
		return Prim(KindStr), nil
	case "bytes":
		return Prim(KindBytes), nil
	case "tensors":
		return Prim(KindTensors), nil
	}
	if inner, ok := unwrap(s, "list<", ">"); ok {
		e, err := Parse(inner)
		if err != nil {
			return nil, err
		}
		return ListOf(e), nil
	}
	if inner, ok := unwrap(s, "opt<", ">"); ok {
		e, err := Parse(inner)
		if err != nil {
			return nil, err
		}
		return OptOf(e), nil
	}
	if inner, ok := unwrap(s, "dict<str,", ">"); ok {
		e, err := Parse(inner)
		if err != nil {
			return nil, err
		}
		return DictOf(e), nil
	}
	if body, ok := unwrap(s, "struct{", "}"); ok {
		if body == "" {
			return StructOf(), nil
		}
		var fields []Field
		for _, part := range strings.Split(body, ",") {
			if part == "" {
				continue
			}
			name, typ, ok := strings.Cut(part, ":")
			if !ok || name == "" {
				return nil, fmt.Errorf("kiln: bad field %q", part)
			}
			ft, err := Parse(typ)
			if err != nil {
				return nil, err
			}
			fields = append(fields, Field{Name: name, Type: ft})
		}
		return StructOf(fields...), nil
	}
	return nil, fmt.Errorf("kiln: unknown type %q", s)
}

func unwrap(s, pre, suf string) (string, bool) {
	if strings.HasPrefix(s, pre) && strings.HasSuffix(s, suf) && len(s) >= len(pre)+len(suf) {
		return s[len(pre) : len(s)-len(suf)], true
	}
	return "", false
}

// Options tunes a decode. Cap 0 selects DefaultCap. Trusted skips UTF-8 and quant-shape checks.
//
// Numeric lists decode to typed slices. Scratch, when set, is used for the
// first list of each kind whose capacity is large enough; those slices are
// not pooled. Pool reuses list storage across decodes. A nil Pool selects
// the package pool. A slice drawn from a pool is stable only until the next
// decode on that pool that materializes another pooled list.
type Options struct {
	Cap     int
	Trusted bool
	Scratch *ListScratch
	Pool    *ListPool
}

func (o Options) cap() int {
	if o.Cap <= 0 {
		return DefaultCap
	}
	return o.Cap
}

func mix(h uint64, data string) uint64 {
	for i := 0; i < len(data); i++ {
		h ^= uint64(data[i])
		h *= fnvPrime
	}
	h *= fnvPrime
	return h
}

func hashSchema(h uint64, s *Schema) uint64 {
	switch s.Kind {
	case KindBool:
		return mix(h, "bool")
	case KindI8:
		return mix(h, "i8")
	case KindI16:
		return mix(h, "i16")
	case KindI32:
		return mix(h, "i32")
	case KindI64:
		return mix(h, "i64")
	case KindInt:
		return mix(h, "int")
	case KindU8:
		return mix(h, "u8")
	case KindU16:
		return mix(h, "u16")
	case KindU32:
		return mix(h, "u32")
	case KindU64:
		return mix(h, "u64")
	case KindUint:
		return mix(h, "uint")
	case KindF32:
		return mix(h, "f32")
	case KindF64:
		return mix(h, "f64")
	case KindStr:
		return mix(h, "str")
	case KindBytes:
		return mix(h, "bytes")
	case KindTensors:
		return mix(h, "tensors")
	case KindList:
		h = mix(h, "list<")
		h = hashSchema(h, s.Elem)
		return mix(h, ">")
	case KindOpt:
		h = mix(h, "opt<")
		h = hashSchema(h, s.Elem)
		return mix(h, ">")
	case KindDict:
		h = mix(h, "dict<str,")
		h = hashSchema(h, s.Elem)
		return mix(h, ">")
	case KindStruct:
		h = mix(h, "struct{")
		for _, f := range s.Fields {
			h = mix(h, f.Name)
			h = mix(h, ":")
			h = hashSchema(h, f.Type)
			h = mix(h, ",")
		}
		return mix(h, "}")
	default:
		return h
	}
}

// TypeHash is the FNV-1a 64 schema hash, including the trailing 0x00 mix on each token.
func TypeHash(s *Schema) uint64 { return hashSchema(fnvOffset, s) }

// MinSize is the minimum wire size of one value of s.
func MinSize(s *Schema) (int, error) {
	switch s.Kind {
	case KindBool, KindI8, KindU8:
		return 1, nil
	case KindI16, KindU16:
		return 2, nil
	case KindI32, KindU32, KindF32:
		return 4, nil
	case KindI64, KindU64, KindInt, KindUint, KindF64:
		return 8, nil
	case KindStr, KindBytes, KindList, KindDict:
		return 4, nil
	case KindOpt:
		return 1, nil
	case KindTensors:
		return 64, nil
	case KindStruct:
		n := 0
		for _, f := range s.Fields {
			m, err := MinSize(f.Type)
			if err != nil {
				return 0, err
			}
			n += m
		}
		return n, nil
	default:
		return 0, fmt.Errorf("kiln: unknown schema")
	}
}

// CheckCount reports whether count elements of size each fit in remain unread bytes.
// It does not allocate. each 0 skips the product check; the cap still applies.
func CheckCount(remain int, count uint32, each int, cap int) error {
	if cap <= 0 {
		cap = DefaultCap
	}
	if uint64(count) > uint64(cap) {
		return Fail("LIMIT")
	}
	if each != 0 {
		if uint64(count) > ^uint64(0)/uint64(each) {
			return Fail("LIMIT")
		}
		if uint64(count)*uint64(each) > uint64(remain) {
			return Fail("EOF")
		}
	}
	return nil
}

func Encode(s *Schema, v any) ([]byte, error) { return Append(nil, s, v) }

func Append(dst []byte, s *Schema, v any) ([]byte, error) {
	switch s.Kind {
	case KindBool:
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("kiln: want bool")
		}
		if b {
			return append(dst, 1), nil
		}
		return append(dst, 0), nil
	case KindI8:
		n, err := asInt64(v)
		if err != nil || n < math.MinInt8 || n > math.MaxInt8 {
			return nil, fmt.Errorf("kiln: want i8")
		}
		return append(dst, byte(int8(n))), nil
	case KindU8:
		n, err := asUint64(v)
		if err != nil || n > math.MaxUint8 {
			return nil, fmt.Errorf("kiln: want u8")
		}
		return append(dst, byte(n)), nil
	case KindI16:
		n, err := asInt64(v)
		if err != nil || n < math.MinInt16 || n > math.MaxInt16 {
			return nil, fmt.Errorf("kiln: want i16")
		}
		return binary.LittleEndian.AppendUint16(dst, uint16(int16(n))), nil
	case KindU16:
		n, err := asUint64(v)
		if err != nil || n > math.MaxUint16 {
			return nil, fmt.Errorf("kiln: want u16")
		}
		return binary.LittleEndian.AppendUint16(dst, uint16(n)), nil
	case KindI32:
		n, err := asInt64(v)
		if err != nil || n < math.MinInt32 || n > math.MaxInt32 {
			return nil, fmt.Errorf("kiln: want i32")
		}
		return binary.LittleEndian.AppendUint32(dst, uint32(int32(n))), nil
	case KindU32:
		n, err := asUint64(v)
		if err != nil || n > math.MaxUint32 {
			return nil, fmt.Errorf("kiln: want u32")
		}
		return binary.LittleEndian.AppendUint32(dst, uint32(n)), nil
	case KindI64, KindInt:
		n, err := asInt64(v)
		if err != nil {
			return nil, err
		}
		return binary.LittleEndian.AppendUint64(dst, uint64(n)), nil
	case KindU64, KindUint:
		n, err := asUint64(v)
		if err != nil {
			return nil, err
		}
		return binary.LittleEndian.AppendUint64(dst, n), nil
	case KindF32:
		bits, err := f32bits(v)
		if err != nil {
			return nil, err
		}
		return binary.LittleEndian.AppendUint32(dst, bits), nil
	case KindF64:
		bits, err := f64bits(v)
		if err != nil {
			return nil, err
		}
		return binary.LittleEndian.AppendUint64(dst, bits), nil
	case KindStr:
		str, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("kiln: want string")
		}
		return appendCounted(dst, []byte(str))
	case KindBytes:
		b, ok := v.([]byte)
		if !ok {
			return nil, fmt.Errorf("kiln: want bytes")
		}
		return appendCounted(dst, b)
	case KindList:
		return appendList(dst, s.Elem, v)
	case KindOpt:
		if v == nil {
			return append(dst, 0), nil
		}
		dst = append(dst, 1)
		return Append(dst, s.Elem, v)
	case KindDict:
		return appendDict(dst, s.Elem, v)
	case KindStruct:
		if rec, ok := v.(*Record); ok {
			return appendScalar(dst, s, rec), nil
		}
		m, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("kiln: want struct map")
		}
		if s.fast == 1 {
			return appendScalarMap(dst, s, m)
		}
		var err error
		for _, f := range s.Fields {
			fv, ok := m[f.Name]
			if !ok || fv == nil {
				if f.Type.Kind == KindOpt {
					dst = append(dst, 0)
					continue
				}
				if !ok {
					return nil, fmt.Errorf("kiln: missing field %s", f.Name)
				}
			}
			dst, err = Append(dst, f.Type, fv)
			if err != nil {
				return nil, err
			}
		}
		return dst, nil
	case KindTensors:
		return appendTensors(dst, v)
	default:
		return nil, fmt.Errorf("kiln: unknown schema")
	}
}

func appendCounted(dst, raw []byte) ([]byte, error) {
	if len(raw) > math.MaxUint32 {
		return nil, Fail("LIMIT")
	}
	dst = binary.LittleEndian.AppendUint32(dst, uint32(len(raw)))
	return append(dst, raw...), nil
}

func appendList(dst []byte, elem *Schema, v any) ([]byte, error) {
	if xs, ok := v.([]any); ok {
		return appendAnyList(dst, elem, xs)
	}
	switch elem.Kind {
	case KindU8:
		b, ok := asBytes(v)
		if !ok {
			return nil, fmt.Errorf("kiln: want list<u8>")
		}
		return appendCounted(dst, b)
	case KindI8:
		if b, ok := v.([]byte); ok {
			return appendCounted(dst, b)
		}
		x, ok := v.([]int8)
		if !ok {
			return nil, fmt.Errorf("kiln: want list<i8>")
		}
		return appendCount(dst, x, appendI8s)
	case KindI16:
		x, ok := v.([]int16)
		if !ok {
			return nil, fmt.Errorf("kiln: want list<i16>")
		}
		return appendCount(dst, x, appendI16s)
	case KindU16:
		x, ok := v.([]uint16)
		if !ok {
			return nil, fmt.Errorf("kiln: want list<u16>")
		}
		return appendCount(dst, x, appendU16s)
	case KindI32:
		x, ok := v.([]int32)
		if !ok {
			return nil, fmt.Errorf("kiln: want list<i32>")
		}
		return appendCount(dst, x, appendI32s)
	case KindU32:
		x, ok := v.([]uint32)
		if !ok {
			return nil, fmt.Errorf("kiln: want list<u32>")
		}
		return appendCount(dst, x, appendU32s)
	case KindI64:
		x, ok := v.([]int64)
		if !ok {
			return nil, fmt.Errorf("kiln: want list<i64>")
		}
		return appendCount(dst, x, appendI64s)
	case KindInt:
		x, ok := v.([]int)
		if !ok {
			return nil, fmt.Errorf("kiln: want list<int>")
		}
		return appendCount(dst, x, appendInts)
	case KindU64:
		x, ok := v.([]uint64)
		if !ok {
			return nil, fmt.Errorf("kiln: want list<u64>")
		}
		return appendCount(dst, x, appendU64s)
	case KindUint:
		x, ok := v.([]uint)
		if !ok {
			return nil, fmt.Errorf("kiln: want list<uint>")
		}
		return appendCount(dst, x, appendUints)
	case KindF32:
		x, ok := v.([]float32)
		if !ok {
			return nil, fmt.Errorf("kiln: want list<f32>")
		}
		if uint64(len(x)) > math.MaxUint32 {
			return nil, Fail("LIMIT")
		}
		dst = putU32(dst, uint32(len(x)))
		return appendF32s(dst, x), nil
	case KindF64:
		x, ok := v.([]float64)
		if !ok {
			return nil, fmt.Errorf("kiln: want list<f64>")
		}
		return appendCount(dst, x, appendF64s)
	default:
		return nil, fmt.Errorf("kiln: want list")
	}
}

func appendAnyList(dst []byte, elem *Schema, x []any) ([]byte, error) {
	if uint64(len(x)) > math.MaxUint32 {
		return nil, Fail("LIMIT")
	}
	dst = putU32(dst, uint32(len(x)))
	var err error
	for _, item := range x {
		dst, err = Append(dst, elem, item)
		if err != nil {
			return nil, err
		}
	}
	return dst, nil
}

func appendDict(dst []byte, elem *Schema, v any) ([]byte, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("kiln: want dict")
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	dst = binary.LittleEndian.AppendUint32(dst, uint32(len(keys)))
	var err error
	for _, k := range keys {
		dst, err = appendCounted(dst, []byte(k))
		if err != nil {
			return nil, err
		}
		dst, err = Append(dst, elem, m[k])
		if err != nil {
			return nil, err
		}
	}
	return dst, nil
}

func EncodeFramed(s *Schema, v any) ([]byte, error) {
	body, err := Encode(s, v)
	if err != nil {
		return nil, err
	}
	dst := make([]byte, 16+len(body))
	copy(dst[:4], "KLN0")
	binary.LittleEndian.PutUint64(dst[8:16], TypeHash(s))
	copy(dst[16:], body)
	return dst, nil
}

func asBytes(v any) ([]byte, bool) {
	switch x := v.(type) {
	case []byte:
		return x, true
	case string:
		return []byte(x), true
	default:
		return nil, false
	}
}

func asInt64(v any) (int64, error) {
	switch n := v.(type) {
	case int:
		return int64(n), nil
	case int8:
		return int64(n), nil
	case int16:
		return int64(n), nil
	case int32:
		return int64(n), nil
	case int64:
		return n, nil
	case uint8:
		return int64(n), nil
	case uint16:
		return int64(n), nil
	case uint32:
		return int64(n), nil
	default:
		return 0, fmt.Errorf("kiln: want signed integer")
	}
}

func asUint64(v any) (uint64, error) {
	switch n := v.(type) {
	case uint8:
		return uint64(n), nil
	case uint16:
		return uint64(n), nil
	case uint32:
		return uint64(n), nil
	case uint64:
		return n, nil
	case int:
		if n < 0 {
			return 0, fmt.Errorf("kiln: negative unsigned")
		}
		return uint64(n), nil
	case int64:
		if n < 0 {
			return 0, fmt.Errorf("kiln: negative unsigned")
		}
		return uint64(n), nil
	default:
		return 0, fmt.Errorf("kiln: want unsigned integer")
	}
}

func f32bits(v any) (uint32, error) {
	switch n := v.(type) {
	case float32:
		return math.Float32bits(n), nil
	case float64:
		return math.Float32bits(float32(n)), nil
	default:
		return 0, fmt.Errorf("kiln: want f32")
	}
}

func f64bits(v any) (uint64, error) {
	switch n := v.(type) {
	case float64:
		return math.Float64bits(n), nil
	case float32:
		return math.Float64bits(float64(n)), nil
	default:
		return 0, fmt.Errorf("kiln: want f64")
	}
}

type cursor struct {
	b       []byte
	i       int
	capn    int
	trusted bool
	depth   int

	pool      *ListPool
	scratch   *ListScratch
	used      uint32
	recycled  bool
	mine      [4][]byte
	nmine     int
	mineExtra [][]byte
}

func Decode(s *Schema, buf []byte, opt Options) (any, error) {
	v, rest, err := DecodeOne(s, buf, opt)
	if err != nil {
		return nil, err
	}
	if len(rest) != 0 {
		return nil, Fail("TRAILING")
	}
	return v, nil
}

// DecodeOne reads one value and returns the unread suffix.
// Numeric lists may alias Options.Pool, or the package pool when Pool is nil.
// Those slices are stable only until the next pooled-list decode on that pool.
func DecodeOne(s *Schema, buf []byte, opt Options) (any, []byte, error) {
	d := cursor{b: buf, capn: opt.cap(), trusted: opt.Trusted, pool: opt.Pool, scratch: opt.Scratch}
	defer d.finish()
	v, err := d.val(s)
	if err != nil {
		return nil, nil, err
	}
	return v, d.b[d.i:], nil
}

func DecodeFramed(s *Schema, buf []byte, opt Options) (uint64, any, error) {
	if len(buf) < 4 {
		return 0, nil, Fail("EOF")
	}
	if buf[0] != 'K' || buf[1] != 'L' || buf[2] != 'N' || buf[3] != '0' {
		return 0, nil, Fail("FRAME")
	}
	if len(buf) < 16 {
		return 0, nil, Fail("EOF")
	}
	if buf[4]|buf[5]|buf[6]|buf[7] != 0 {
		return 0, nil, Fail("FRAME")
	}
	hash := binary.LittleEndian.Uint64(buf[8:16])
	if hash != TypeHash(s) {
		return 0, nil, Fail("SCHEMA")
	}
	v, err := Decode(s, buf[16:], opt)
	if err != nil {
		return 0, nil, err
	}
	return hash, v, nil
}

func (d *cursor) need(n int) error {
	if n < 0 || len(d.b)-d.i < n {
		return Fail("EOF")
	}
	return nil
}

func (d *cursor) enter() error {
	d.depth++
	if d.depth > maxDepth {
		return Fail("LIMIT")
	}
	return nil
}

func (d *cursor) val(s *Schema) (any, error) {
	switch s.Kind {
	case KindBool:
		if err := d.need(1); err != nil {
			return nil, err
		}
		b := d.b[d.i]
		d.i++
		if b > 1 {
			return nil, Fail("TYPE")
		}
		return b == 1, nil
	case KindI8:
		if err := d.need(1); err != nil {
			return nil, err
		}
		n := int8(d.b[d.i])
		d.i++
		return n, nil
	case KindU8:
		if err := d.need(1); err != nil {
			return nil, err
		}
		n := d.b[d.i]
		d.i++
		return n, nil
	case KindI16:
		if err := d.need(2); err != nil {
			return nil, err
		}
		n := int16(binary.LittleEndian.Uint16(d.b[d.i:]))
		d.i += 2
		return n, nil
	case KindU16:
		if err := d.need(2); err != nil {
			return nil, err
		}
		n := binary.LittleEndian.Uint16(d.b[d.i:])
		d.i += 2
		return n, nil
	case KindI32:
		if err := d.need(4); err != nil {
			return nil, err
		}
		n := int32(binary.LittleEndian.Uint32(d.b[d.i:]))
		d.i += 4
		return n, nil
	case KindU32:
		if err := d.need(4); err != nil {
			return nil, err
		}
		n := binary.LittleEndian.Uint32(d.b[d.i:])
		d.i += 4
		return n, nil
	case KindI64, KindInt:
		if err := d.need(8); err != nil {
			return nil, err
		}
		n := int64(binary.LittleEndian.Uint64(d.b[d.i:]))
		d.i += 8
		return n, nil
	case KindU64, KindUint:
		if err := d.need(8); err != nil {
			return nil, err
		}
		n := binary.LittleEndian.Uint64(d.b[d.i:])
		d.i += 8
		return n, nil
	case KindF32:
		if err := d.need(4); err != nil {
			return nil, err
		}
		n := math.Float32frombits(binary.LittleEndian.Uint32(d.b[d.i:]))
		d.i += 4
		return n, nil
	case KindF64:
		if err := d.need(8); err != nil {
			return nil, err
		}
		n := math.Float64frombits(binary.LittleEndian.Uint64(d.b[d.i:]))
		d.i += 8
		return n, nil
	case KindStr:
		return d.str()
	case KindBytes:
		return d.blob()
	case KindList:
		return d.list(s.Elem)
	case KindOpt:
		return d.opt(s.Elem)
	case KindDict:
		return d.dict(s.Elem)
	case KindStruct:
		return d.struc(s)
	case KindTensors:
		return d.tensors()
	default:
		return nil, fmt.Errorf("kiln: unknown schema")
	}
}

func (d *cursor) u32() (uint32, error) {
	if err := d.need(4); err != nil {
		return 0, err
	}
	n := binary.LittleEndian.Uint32(d.b[d.i:])
	d.i += 4
	return n, nil
}

func (d *cursor) str() (string, error) {
	n, err := d.u32()
	if err != nil {
		return "", err
	}
	if uint64(n) > uint64(d.capn) {
		return "", Fail("LIMIT")
	}
	if err := d.need(int(n)); err != nil {
		return "", err
	}
	raw := d.b[d.i : d.i+int(n)]
	d.i += int(n)
	if !d.trusted && !utf8.Valid(raw) {
		return "", Fail("UTF8")
	}
	return string(raw), nil
}

func (d *cursor) blob() ([]byte, error) {
	n, err := d.u32()
	if err != nil {
		return nil, err
	}
	if uint64(n) > uint64(d.capn) {
		return nil, Fail("LIMIT")
	}
	if err := d.need(int(n)); err != nil {
		return nil, err
	}
	raw := d.b[d.i : d.i+int(n)]
	d.i += int(n)
	out := make([]byte, len(raw))
	copy(out, raw)
	return out, nil
}

func (d *cursor) list(elem *Schema) (any, error) {
	if err := d.enter(); err != nil {
		return nil, err
	}
	defer func() { d.depth-- }()
	n, err := d.u32()
	if err != nil {
		return nil, err
	}
	each, err := MinSize(elem)
	if err != nil {
		return nil, err
	}
	if err := CheckCount(len(d.b)-d.i, n, each, d.capn); err != nil {
		return nil, err
	}
	switch elem.Kind {
	case KindU8:
		return d.u8List(int(n))
	case KindI8:
		return d.i8List(int(n))
	case KindI16:
		return d.i16List(int(n))
	case KindU16:
		return d.u16List(int(n))
	case KindI32:
		return d.i32List(int(n))
	case KindU32:
		return d.u32List(int(n))
	case KindI64:
		return d.i64List(int(n))
	case KindInt:
		return d.intList(int(n))
	case KindU64:
		return d.u64List(int(n))
	case KindUint:
		return d.uintList(int(n))
	case KindF32:
		return d.f32List(int(n))
	case KindF64:
		return d.f64List(int(n))
	}
	out := make([]any, n)
	for i := range out {
		out[i], err = d.val(elem)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (d *cursor) opt(elem *Schema) (any, error) {
	if err := d.enter(); err != nil {
		return nil, err
	}
	defer func() { d.depth-- }()
	if err := d.need(1); err != nil {
		return nil, err
	}
	tag := d.b[d.i]
	d.i++
	if tag == 0 {
		return nil, nil
	}
	if tag != 1 {
		return nil, Fail("TYPE")
	}
	return d.val(elem)
}

func (d *cursor) dict(elem *Schema) (any, error) {
	if err := d.enter(); err != nil {
		return nil, err
	}
	defer func() { d.depth-- }()
	n, err := d.u32()
	if err != nil {
		return nil, err
	}
	each, err := MinSize(elem)
	if err != nil {
		return nil, err
	}
	if err := CheckCount(len(d.b)-d.i, n, 4+each, d.capn); err != nil {
		return nil, err
	}
	out := make(map[string]any, n)
	var prev string
	var have bool
	for i := uint32(0); i < n; i++ {
		k, err := d.str()
		if err != nil {
			return nil, err
		}
		if have && k <= prev {
			return nil, Fail("TYPE")
		}
		prev, have = k, true
		val, err := d.val(elem)
		if err != nil {
			return nil, err
		}
		out[k] = val
	}
	return out, nil
}

func appendScalarMap(dst []byte, s *Schema, m map[string]any) ([]byte, error) {
	for _, f := range s.Fields {
		fv, ok := m[f.Name]
		if !ok {
			return nil, fmt.Errorf("kiln: missing field %s", f.Name)
		}
		switch f.Type.Kind {
		case KindI32:
			if n, ok := fv.(int32); ok {
				dst = putU32(dst, uint32(n))
				continue
			}
		case KindI64, KindInt:
			if n, ok := fv.(int64); ok {
				dst = putU64(dst, uint64(n))
				continue
			}
		case KindF64:
			if n, ok := fv.(float64); ok {
				dst = putU64(dst, math.Float64bits(n))
				continue
			}
		case KindBool:
			if n, ok := fv.(bool); ok {
				dst = PutBool(dst, n)
				continue
			}
		}
		bits, err := valueBits(f.Type.Kind, fv)
		if err != nil {
			return nil, err
		}
		dst = appendBits(dst, f.Type.Kind, bits)
	}
	return dst, nil
}

func (d *cursor) struc(s *Schema) (any, error) {
	d.depth++
	if d.depth > maxDepth {
		return nil, Fail("LIMIT")
	}
	if s.fast == 1 {
		rec := &Record{sch: s}
		for i, f := range s.Fields {
			bits, err := d.readBits(f.Type.Kind)
			if err != nil {
				d.depth--
				return nil, err
			}
			rec.bits[i] = bits
		}
		d.depth--
		return rec, nil
	}
	out := make(map[string]any, len(s.Fields))
	for _, f := range s.Fields {
		v, err := d.val(f.Type)
		if err != nil {
			d.depth--
			return nil, err
		}
		out[f.Name] = v
	}
	d.depth--
	return out, nil
}
