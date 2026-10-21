package kiln

import (
	"encoding/binary"
	"math"
	"unsafe"
)

func appendRaw(dst, raw []byte) []byte {
	if len(raw) == 0 {
		return dst
	}
	n := len(dst)
	// Large payloads are faster to copy when the first byte is 32-byte aligned.
	// runtime.memmove then uses an aligned quadword loop.
	if len(raw) >= 1<<18 {
		dst = alignPayload(dst, n+len(raw), 32)
	} else {
		dst = grow(dst, len(raw))
	}
	copy(dst[n:], raw)
	return dst
}

// alignPayload returns a slice with the same length and contents as dst whose
// index len(dst) is align-aligned, and whose capacity is at least need.
func alignPayload(dst []byte, need, align int) []byte {
	if cap(dst) >= need && len(dst) > 0 {
		if (uintptr(unsafe.Pointer(&dst[0]))+uintptr(len(dst)))%uintptr(align) == 0 {
			return dst[:need]
		}
	}
	if cap(dst) >= need && len(dst) == 0 && cap(dst) > 0 {
		if uintptr(unsafe.Pointer(&dst[:1][0]))%uintptr(align) == 0 {
			return dst[:need]
		}
	}
	extra := align
	buf := allocBytes(need + extra)
	base := uintptr(unsafe.Pointer(&buf[0]))
	off := int((uintptr(align) - (base+uintptr(len(dst)))%uintptr(align)) % uintptr(align))
	out := buf[off : off+len(dst) : off+need]
	copy(out, dst)
	return out[:need]
}

// appendLE copies x as little-endian wire bytes when the host layout matches.
// A false result means the caller must write element by element.
func appendLE[T any](dst []byte, x []T, width int) ([]byte, bool) {
	var zero T
	if int(unsafe.Sizeof(zero)) != width || (width != 1 && !hostLE) {
		return dst, false
	}
	raw := unsafe.Slice((*byte)(unsafe.Pointer(&x[0])), len(x)*width)
	return appendRaw(dst, raw), true
}

func copyLE[T any](dst []T, src []byte, width int) bool {
	if len(dst) == 0 {
		return true
	}
	var zero T
	if int(unsafe.Sizeof(zero)) != width || (width != 1 && !hostLE) {
		return false
	}
	raw := unsafe.Slice((*byte)(unsafe.Pointer(&dst[0])), len(dst)*width)
	if len(raw) >= 1<<18 && uintptr(unsafe.Pointer(&raw[0]))&31 == 0 {
		copyNT(unsafe.Pointer(&raw[0]), unsafe.Pointer(&src[0]), uintptr(len(raw)))
		return true
	}
	copy(raw, src)
	return true
}

func appendI8s(dst []byte, x []int8) []byte {
	if len(x) == 0 {
		return dst
	}
	if out, ok := appendLE(dst, x, 1); ok {
		return out
	}
	for _, n := range x {
		dst = append(dst, byte(n))
	}
	return dst
}

func appendU8s(dst []byte, x []byte) []byte { return appendRaw(dst, x) }

func appendI16s(dst []byte, x []int16) []byte {
	if len(x) == 0 {
		return dst
	}
	if out, ok := appendLE(dst, x, 2); ok {
		return out
	}
	for _, n := range x {
		dst = putU16(dst, uint16(n))
	}
	return dst
}

func appendU16s(dst []byte, x []uint16) []byte {
	if len(x) == 0 {
		return dst
	}
	if out, ok := appendLE(dst, x, 2); ok {
		return out
	}
	for _, n := range x {
		dst = putU16(dst, n)
	}
	return dst
}

func appendI32s(dst []byte, x []int32) []byte {
	if len(x) == 0 {
		return dst
	}
	if out, ok := appendLE(dst, x, 4); ok {
		return out
	}
	for _, n := range x {
		dst = putU32(dst, uint32(n))
	}
	return dst
}

func appendU32s(dst []byte, x []uint32) []byte {
	if len(x) == 0 {
		return dst
	}
	if out, ok := appendLE(dst, x, 4); ok {
		return out
	}
	for _, n := range x {
		dst = putU32(dst, n)
	}
	return dst
}

func appendI64s(dst []byte, x []int64) []byte {
	if len(x) == 0 {
		return dst
	}
	if out, ok := appendLE(dst, x, 8); ok {
		return out
	}
	for _, n := range x {
		dst = putU64(dst, uint64(n))
	}
	return dst
}

func appendU64s(dst []byte, x []uint64) []byte {
	if len(x) == 0 {
		return dst
	}
	if out, ok := appendLE(dst, x, 8); ok {
		return out
	}
	for _, n := range x {
		dst = putU64(dst, n)
	}
	return dst
}

func appendInts(dst []byte, x []int) []byte {
	if len(x) == 0 {
		return dst
	}
	if out, ok := appendLE(dst, x, 8); ok {
		return out
	}
	for _, n := range x {
		dst = putU64(dst, uint64(n))
	}
	return dst
}

func appendUints(dst []byte, x []uint) []byte {
	if len(x) == 0 {
		return dst
	}
	if out, ok := appendLE(dst, x, 8); ok {
		return out
	}
	for _, n := range x {
		dst = putU64(dst, uint64(n))
	}
	return dst
}

func readI8s(dst []int8, src []byte) {
	if copyLE(dst, src, 1) {
		return
	}
	for i := range dst {
		dst[i] = int8(src[i])
	}
}

func readU8s(dst []byte, src []byte) { copy(dst, src) }

func readI16s(dst []int16, src []byte) {
	if copyLE(dst, src, 2) {
		return
	}
	for i := range dst {
		dst[i] = int16(binary.LittleEndian.Uint16(src[i*2:]))
	}
}

func readU16s(dst []uint16, src []byte) {
	if copyLE(dst, src, 2) {
		return
	}
	for i := range dst {
		dst[i] = binary.LittleEndian.Uint16(src[i*2:])
	}
}

func readI32s(dst []int32, src []byte) {
	if copyLE(dst, src, 4) {
		return
	}
	for i := range dst {
		dst[i] = int32(binary.LittleEndian.Uint32(src[i*4:]))
	}
}

func readU32s(dst []uint32, src []byte) {
	if copyLE(dst, src, 4) {
		return
	}
	for i := range dst {
		dst[i] = binary.LittleEndian.Uint32(src[i*4:])
	}
}

func readI64s(dst []int64, src []byte) {
	if copyLE(dst, src, 8) {
		return
	}
	for i := range dst {
		dst[i] = int64(binary.LittleEndian.Uint64(src[i*8:]))
	}
}

func readU64s(dst []uint64, src []byte) {
	if copyLE(dst, src, 8) {
		return
	}
	for i := range dst {
		dst[i] = binary.LittleEndian.Uint64(src[i*8:])
	}
}

func readInts(dst []int, src []byte) {
	if copyLE(dst, src, 8) {
		return
	}
	for i := range dst {
		dst[i] = int(binary.LittleEndian.Uint64(src[i*8:]))
	}
}

func readUints(dst []uint, src []byte) {
	if copyLE(dst, src, 8) {
		return
	}
	for i := range dst {
		dst[i] = uint(binary.LittleEndian.Uint64(src[i*8:]))
	}
}

func readF32s(dst []float32, src []byte) {
	if copyLE(dst, src, 4) {
		return
	}
	for i := range dst {
		dst[i] = math.Float32frombits(binary.LittleEndian.Uint32(src[i*4:]))
	}
}

func readF64s(dst []float64, src []byte) {
	if copyLE(dst, src, 8) {
		return
	}
	for i := range dst {
		dst[i] = math.Float64frombits(binary.LittleEndian.Uint64(src[i*8:]))
	}
}

func takeNums[T any](buf []byte, n uint32, width int, read func([]T, []byte)) ([]T, []byte, error) {
	if err := CheckCount(len(buf), n, width, 0); err != nil {
		return nil, buf, err
	}
	if n == 0 {
		return []T{}, buf, nil
	}
	out := allocSlice[T](int(n))
	read(out, buf)
	return out, buf[int(n)*width:], nil
}

func appendCount[T any](dst []byte, x []T, write func([]byte, []T) []byte) ([]byte, error) {
	if uint64(len(x)) > math.MaxUint32 {
		return nil, Fail("LIMIT")
	}
	dst = putU32(dst, uint32(len(x)))
	return write(dst, x), nil
}

// AppendI8s appends the raw bytes of x. AppendU8s appends x unchanged.
func AppendI8s(dst []byte, x []int8) []byte { return appendI8s(dst, x) }
func AppendU8s(dst []byte, x []byte) []byte { return appendU8s(dst, x) }

// AppendI16s appends little-endian i16 bits. On a little-endian host this is one copy.
func AppendI16s(dst []byte, x []int16) []byte   { return appendI16s(dst, x) }
func AppendU16s(dst []byte, x []uint16) []byte  { return appendU16s(dst, x) }
func AppendI32s(dst []byte, x []int32) []byte   { return appendI32s(dst, x) }
func AppendU32s(dst []byte, x []uint32) []byte  { return appendU32s(dst, x) }
func AppendI64s(dst []byte, x []int64) []byte   { return appendI64s(dst, x) }
func AppendU64s(dst []byte, x []uint64) []byte  { return appendU64s(dst, x) }
func AppendF32s(dst []byte, x []float32) []byte { return appendF32s(dst, x) }
func AppendF64s(dst []byte, x []float64) []byte { return appendF64s(dst, x) }

// ReadF32s copies little-endian f32 bits from src into dst.
// len(src) must be at least 4*len(dst).
func ReadF32s(dst []float32, src []byte) { readF32s(dst, src) }

// TakeF32s reads n f32 values into a new slice the caller owns.
// The slice is not drawn from a list pool. Cap is DefaultCap.
func TakeF32s(buf []byte, n uint32) ([]float32, []byte, error) {
	return takeNums(buf, n, 4, readF32s)
}

func TakeF64s(buf []byte, n uint32) ([]float64, []byte, error) {
	return takeNums(buf, n, 8, readF64s)
}
func TakeI8s(buf []byte, n uint32) ([]int8, []byte, error) {
	return takeNums(buf, n, 1, readI8s)
}
func TakeU8s(buf []byte, n uint32) ([]byte, []byte, error) {
	return takeNums(buf, n, 1, readU8s)
}
func TakeI16s(buf []byte, n uint32) ([]int16, []byte, error) {
	return takeNums(buf, n, 2, readI16s)
}
func TakeU16s(buf []byte, n uint32) ([]uint16, []byte, error) {
	return takeNums(buf, n, 2, readU16s)
}
func TakeI32s(buf []byte, n uint32) ([]int32, []byte, error) {
	return takeNums(buf, n, 4, readI32s)
}
func TakeU32s(buf []byte, n uint32) ([]uint32, []byte, error) {
	return takeNums(buf, n, 4, readU32s)
}
func TakeI64s(buf []byte, n uint32) ([]int64, []byte, error) {
	return takeNums(buf, n, 8, readI64s)
}
func TakeU64s(buf []byte, n uint32) ([]uint64, []byte, error) {
	return takeNums(buf, n, 8, readU64s)
}
