package kiln

import "unsafe"

var useAVX2 bool

func init() {
	useAVX2 = avx2Enabled()
}

func avx2Enabled() bool

func copyNTSSE(dst, src unsafe.Pointer, n uintptr)
func copyNTAVX2(dst, src unsafe.Pointer, n uintptr)

func copyNT(dst, src unsafe.Pointer, n uintptr) {
	if useAVX2 && n >= 128 && uintptr(dst)&31 == 0 {
		copyNTAVX2(dst, src, n)
		return
	}
	copyNTSSE(dst, src, n)
}
