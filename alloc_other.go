//go:build !linux

package kiln

func allocMaybeHuge(n int) []byte { return allocBytes(n) }
