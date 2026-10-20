package kiln

import (
	"encoding/binary"
	"math"
)

// Record is a decoded struct of at most eight scalar fields.
// Numeric fields stay unboxed. Get materializes one field.
type Record struct {
	sch  *Schema
	bits [8]uint64
}

// Get returns the field value, or false when name is not a field.
func (r *Record) Get(name string) (any, bool) {
	if r == nil || r.sch == nil {
		return nil, false
	}
	for i, f := range r.sch.Fields {
		if f.Name == name {
			return bitsToValue(f.Type.Kind, r.bits[i]), true
		}
	}
	return nil, false
}

func scalarKind(k Kind) bool {
	switch k {
	case KindBool, KindI8, KindI16, KindI32, KindI64, KindInt,
		KindU8, KindU16, KindU32, KindU64, KindUint, KindF32, KindF64:
		return true
	default:
		return false
	}
}

func pureScalar(s *Schema) bool {
	n := len(s.Fields)
	if n == 0 || n > 8 {
		return false
	}
	for _, f := range s.Fields {
		if f.Type == nil || !scalarKind(f.Type.Kind) {
			return false
		}
	}
	return true
}

func (d *cursor) readBits(k Kind) (uint64, error) {
	switch k {
	case KindBool:
		if err := d.need(1); err != nil {
			return 0, err
		}
		b := d.b[d.i]
		d.i++
		if b > 1 {
			return 0, Fail("TYPE")
		}
		return uint64(b), nil
	case KindI8, KindU8:
		if err := d.need(1); err != nil {
			return 0, err
		}
		b := d.b[d.i]
		d.i++
		return uint64(b), nil
	case KindI16, KindU16:
		if err := d.need(2); err != nil {
			return 0, err
		}
		n := binary.LittleEndian.Uint16(d.b[d.i:])
		d.i += 2
		return uint64(n), nil
	case KindI32, KindU32, KindF32:
		if err := d.need(4); err != nil {
			return 0, err
		}
		n := binary.LittleEndian.Uint32(d.b[d.i:])
		d.i += 4
		return uint64(n), nil
	case KindI64, KindInt, KindU64, KindUint, KindF64:
		if err := d.need(8); err != nil {
			return 0, err
		}
		n := binary.LittleEndian.Uint64(d.b[d.i:])
		d.i += 8
		return n, nil
	default:
		return 0, Fail("TYPE")
	}
}

func appendBits(dst []byte, k Kind, bits uint64) []byte {
	switch k {
	case KindBool:
		return append(dst, byte(bits))
	case KindI8, KindU8:
		return append(dst, byte(bits))
	case KindI16, KindU16:
		return putU16(dst, uint16(bits))
	case KindI32, KindU32, KindF32:
		return putU32(dst, uint32(bits))
	default:
		return putU64(dst, bits)
	}
}

func appendScalar(dst []byte, s *Schema, rec *Record) []byte {
	for i, f := range s.Fields {
		dst = appendBits(dst, f.Type.Kind, rec.bits[i])
	}
	return dst
}

func valueBits(k Kind, v any) (uint64, error) {
	switch k {
	case KindBool:
		b, ok := v.(bool)
		if !ok {
			return 0, Fail("TYPE")
		}
		if b {
			return 1, nil
		}
		return 0, nil
	case KindI8:
		n, err := asInt64(v)
		if err != nil || n < -128 || n > 127 {
			return 0, Fail("TYPE")
		}
		return uint64(uint8(int8(n))), nil
	case KindI16:
		n, err := asInt64(v)
		if err != nil || n < -32768 || n > 32767 {
			return 0, Fail("TYPE")
		}
		return uint64(uint16(int16(n))), nil
	case KindI32:
		n, err := asInt64(v)
		if err != nil || n < -1<<31 || n > 1<<31-1 {
			return 0, Fail("TYPE")
		}
		return uint64(uint32(int32(n))), nil
	case KindI64, KindInt:
		n, err := asInt64(v)
		if err != nil {
			return 0, err
		}
		return uint64(n), nil
	case KindU8:
		n, err := asUint64(v)
		if err != nil || n > 255 {
			return 0, Fail("TYPE")
		}
		return n, nil
	case KindU16:
		n, err := asUint64(v)
		if err != nil || n > 65535 {
			return 0, Fail("TYPE")
		}
		return n, nil
	case KindU32:
		n, err := asUint64(v)
		if err != nil || n > 0xFFFFFFFF {
			return 0, Fail("TYPE")
		}
		return n, nil
	case KindU64, KindUint:
		return asUint64(v)
	case KindF32:
		n, err := f32bits(v)
		if err != nil {
			return 0, err
		}
		return uint64(n), nil
	case KindF64:
		return f64bits(v)
	default:
		return 0, Fail("TYPE")
	}
}

func bitsToValue(k Kind, bits uint64) any {
	switch k {
	case KindBool:
		return bits != 0
	case KindI8:
		return int8(bits)
	case KindI16:
		return int16(bits)
	case KindI32:
		return int32(bits)
	case KindI64, KindInt:
		return int64(bits)
	case KindU8:
		return uint8(bits)
	case KindU16:
		return uint16(bits)
	case KindU32:
		return uint32(bits)
	case KindU64, KindUint:
		return bits
	case KindF32:
		return math.Float32frombits(uint32(bits))
	case KindF64:
		return math.Float64frombits(bits)
	default:
		return nil
	}
}
