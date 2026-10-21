//go:build !amd64

package kiln

import "unsafe"

func copyNT(dst, src unsafe.Pointer, n uintptr) {
	copy(unsafe.Slice((*byte)(dst), int(n)), unsafe.Slice((*byte)(src), int(n)))
}
