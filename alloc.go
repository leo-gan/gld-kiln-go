package kiln

import "unsafe"

//go:linkname mallocgc runtime.mallocgc
func mallocgc(size uintptr, typ unsafe.Pointer, needzero bool) unsafe.Pointer

// allocBytes returns length-n uninitialized memory with no pointers.
// The caller overwrites every byte it keeps. Small requests are padded
// past the runtime tiny allocator so the slice has a real object.
func allocBytes(n int) []byte {
	if n <= 0 {
		return []byte{}
	}
	size := n
	if size < 32 {
		size = 32
	}
	p := mallocgc(uintptr(size), nil, false)
	return unsafe.Slice((*byte)(p), size)[:n]
}

func allocSlice[T any](n int) []T {
	if n <= 0 {
		return []T{}
	}
	var zero T
	w := int(unsafe.Sizeof(zero))
	raw := allocBytes(n * w)
	return unsafe.Slice((*T)(unsafe.Pointer(&raw[0])), n)
}
