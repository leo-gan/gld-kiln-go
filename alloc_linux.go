//go:build linux

package kiln

import "syscall"

// allocMaybeHuge is the backing store for a large pooled list.
// MADV_HUGEPAGE lets the kernel use 2 MiB pages so a 4 MiB copy
// does not walk a thousand TLB entries. The bytes are faulted once here.
func allocMaybeHuge(n int) []byte {
	if n < 1<<20 {
		return allocBytes(n)
	}
	size := (n + (1<<21 - 1)) &^ (1<<21 - 1)
	b, err := syscall.Mmap(-1, 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_PRIVATE|syscall.MAP_ANONYMOUS)
	if err != nil || len(b) < n {
		return allocBytes(n)
	}
	_ = syscall.Madvise(b, syscall.MADV_HUGEPAGE)
	for i := 0; i < len(b); i += 4096 {
		b[i] = 0
	}
	return b[:n]
}
