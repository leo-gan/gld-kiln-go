package kiln

import (
	"encoding/binary"
	"math"
	"unicode/utf8"
	"unsafe"
)

var hostLE bool

func init() {
	var x uint16 = 0x0001
	hostLE = *(*byte)(unsafe.Pointer(&x)) == 1
}

func appendF32s(dst []byte, x []float32) []byte {
	if len(x) == 0 {
		return dst
	}
	if hostLE {
		raw := unsafe.Slice((*byte)(unsafe.Pointer(&x[0])), len(x)*4)
		return append(dst, raw...)
	}
	for _, n := range x {
		dst = putU32(dst, math.Float32bits(n))
	}
	return dst
}

func appendF64s(dst []byte, x []float64) []byte {
	if len(x) == 0 {
		return dst
	}
	if hostLE {
		raw := unsafe.Slice((*byte)(unsafe.Pointer(&x[0])), len(x)*8)
		return append(dst, raw...)
	}
	for _, n := range x {
		dst = putU64(dst, math.Float64bits(n))
	}
	return dst
}

func readF32s(dst []float32, src []byte) {
	if len(dst) == 0 {
		return
	}
	if hostLE {
		raw := unsafe.Slice((*byte)(unsafe.Pointer(&dst[0])), len(dst)*4)
		copy(raw, src)
		return
	}
	for i := range dst {
		dst[i] = math.Float32frombits(binary.LittleEndian.Uint32(src[i*4:]))
	}
}

func readF64s(dst []float64, src []byte) {
	if len(dst) == 0 {
		return
	}
	if hostLE {
		raw := unsafe.Slice((*byte)(unsafe.Pointer(&dst[0])), len(dst)*8)
		copy(raw, src)
		return
	}
	for i := range dst {
		dst[i] = math.Float64frombits(binary.LittleEndian.Uint64(src[i*8:]))
	}
}

func PutBool(dst []byte, v bool) []byte {
	if v {
		return append(dst, 1)
	}
	return append(dst, 0)
}

func PutI8(dst []byte, v int8) []byte   { return append(dst, byte(v)) }
func PutU8(dst []byte, v uint8) []byte  { return append(dst, v) }
func PutI16(dst []byte, v int16) []byte { return putU16(dst, uint16(v)) }
func PutU16(dst []byte, v uint16) []byte {
	return putU16(dst, v)
}
func PutI32(dst []byte, v int32) []byte { return putU32(dst, uint32(v)) }
func PutU32(dst []byte, v uint32) []byte {
	return putU32(dst, v)
}
func PutI64(dst []byte, v int64) []byte { return putU64(dst, uint64(v)) }
func PutU64(dst []byte, v uint64) []byte {
	return putU64(dst, v)
}
func PutF32(dst []byte, v float32) []byte { return putU32(dst, math.Float32bits(v)) }
func PutF64(dst []byte, v float64) []byte { return putU64(dst, math.Float64bits(v)) }

func putU16(dst []byte, v uint16) []byte {
	n := len(dst)
	dst = grow(dst, 2)
	binary.LittleEndian.PutUint16(dst[n:], v)
	return dst
}

func putU32(dst []byte, v uint32) []byte {
	n := len(dst)
	dst = grow(dst, 4)
	binary.LittleEndian.PutUint32(dst[n:], v)
	return dst
}

func putU64(dst []byte, v uint64) []byte {
	n := len(dst)
	dst = grow(dst, 8)
	binary.LittleEndian.PutUint64(dst[n:], v)
	return dst
}

func grow(dst []byte, n int) []byte {
	if cap(dst)-len(dst) >= n {
		return dst[:len(dst)+n]
	}
	next := cap(dst) * 2
	if next < len(dst)+n {
		next = len(dst) + n
	}
	if next < 64 {
		next = 64
	}
	out := make([]byte, len(dst)+n, next)
	copy(out, dst)
	return out
}

func PutString(dst []byte, v string) ([]byte, error) {
	if len(v) > math.MaxUint32 {
		return nil, Fail("LIMIT")
	}
	dst = putU32(dst, uint32(len(v)))
	return append(dst, v...), nil
}

func PutBytes(dst []byte, v []byte) ([]byte, error) {
	if len(v) > math.MaxUint32 {
		return nil, Fail("LIMIT")
	}
	dst = putU32(dst, uint32(len(v)))
	return append(dst, v...), nil
}

func SizeString(v string) int { return 4 + len(v) }
func SizeBytes(v []byte) int  { return 4 + len(v) }

func TakeBool(buf []byte) (bool, []byte, error) {
	if len(buf) < 1 {
		return false, buf, Fail("EOF")
	}
	if buf[0] > 1 {
		return false, buf, Fail("TYPE")
	}
	return buf[0] == 1, buf[1:], nil
}

func TakeI8(buf []byte) (int8, []byte, error) {
	if len(buf) < 1 {
		return 0, buf, Fail("EOF")
	}
	return int8(buf[0]), buf[1:], nil
}

func TakeU8(buf []byte) (uint8, []byte, error) {
	if len(buf) < 1 {
		return 0, buf, Fail("EOF")
	}
	return buf[0], buf[1:], nil
}

func TakeI16(buf []byte) (int16, []byte, error) {
	n, rest, err := TakeU16(buf)
	return int16(n), rest, err
}

func TakeU16(buf []byte) (uint16, []byte, error) {
	if len(buf) < 2 {
		return 0, buf, Fail("EOF")
	}
	return binary.LittleEndian.Uint16(buf), buf[2:], nil
}

func TakeI32(buf []byte) (int32, []byte, error) {
	n, rest, err := TakeU32(buf)
	return int32(n), rest, err
}

func TakeU32(buf []byte) (uint32, []byte, error) {
	if len(buf) < 4 {
		return 0, buf, Fail("EOF")
	}
	return binary.LittleEndian.Uint32(buf), buf[4:], nil
}

func TakeI64(buf []byte) (int64, []byte, error) {
	n, rest, err := TakeU64(buf)
	return int64(n), rest, err
}

func TakeU64(buf []byte) (uint64, []byte, error) {
	if len(buf) < 8 {
		return 0, buf, Fail("EOF")
	}
	return binary.LittleEndian.Uint64(buf), buf[8:], nil
}

func TakeF32(buf []byte) (float32, []byte, error) {
	n, rest, err := TakeU32(buf)
	return math.Float32frombits(n), rest, err
}

func TakeF64(buf []byte) (float64, []byte, error) {
	n, rest, err := TakeU64(buf)
	return math.Float64frombits(n), rest, err
}

func TakeString(buf []byte, opt Options) (string, []byte, error) {
	raw, rest, err := takeRun(buf, opt)
	if err != nil {
		return "", rest, err
	}
	if !opt.Trusted && !utf8.Valid(raw) {
		return "", rest, Fail("UTF8")
	}
	return string(raw), rest, nil
}

func TakeBytes(buf []byte, opt Options) ([]byte, []byte, error) {
	raw, rest, err := takeRun(buf, opt)
	if err != nil {
		return nil, rest, err
	}
	out := make([]byte, len(raw))
	copy(out, raw)
	return out, rest, nil
}

func takeRun(buf []byte, opt Options) ([]byte, []byte, error) {
	if len(buf) < 4 {
		return nil, buf, Fail("EOF")
	}
	n := binary.LittleEndian.Uint32(buf)
	buf = buf[4:]
	capn := opt.cap()
	if uint64(n) > uint64(capn) {
		return nil, buf, Fail("LIMIT")
	}
	if uint64(n) > uint64(len(buf)) {
		return nil, buf, Fail("EOF")
	}
	return buf[:n], buf[n:], nil
}

func PutOptString(dst []byte, v *string) ([]byte, error) {
	if v == nil {
		return append(dst, 0), nil
	}
	dst = append(dst, 1)
	return PutString(dst, *v)
}

func TakeOptString(buf []byte, opt Options) (*string, []byte, error) {
	if len(buf) < 1 {
		return nil, buf, Fail("EOF")
	}
	switch buf[0] {
	case 0:
		return nil, buf[1:], nil
	case 1:
		s, rest, err := TakeString(buf[1:], opt)
		if err != nil {
			return nil, rest, err
		}
		return &s, rest, nil
	default:
		return nil, buf, Fail("TYPE")
	}
}
