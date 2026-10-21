package kiln

import (
	"sync"
	"unsafe"
)

const (
	useI8 uint32 = 1 << iota
	useU8
	useI16
	useU16
	useI32
	useU32
	useI64
	useU64
	useInt
	useUint
	useF32
	useF64
)

// ListScratch is caller-owned storage for numeric lists.
// The first list of each kind uses the matching field when its capacity
// is large enough. That slice is not stored in a pool.
type ListScratch struct {
	I8   []int8
	U8   []byte
	I16  []int16
	U16  []uint16
	I32  []int32
	U32  []uint32
	I64  []int64
	U64  []uint64
	Int  []int
	Uint []uint
	F32  []float32
	F64  []float64
}

// ListPool retains numeric-list backing arrays between decodes.
// Do not copy a ListPool value.
//
// A slice drawn from the pool aliases storage the pool will reuse.
// It is stable only until the next decode on this pool that materializes
// another pooled list.
type ListPool struct {
	mu   sync.Mutex
	free [][]byte
	held [][]byte
}

var pkgPool ListPool

func (d *cursor) podBytes(n int) []byte {
	if n <= 0 {
		return []byte{}
	}
	p := d.pool
	if p == nil {
		p = &pkgPool
		d.pool = p
	}
	p.mu.Lock()
	if !d.recycled {
		if len(p.held) > 0 {
			p.free = append(p.free, p.held...)
			for i := range p.held {
				p.held[i] = nil
			}
			p.held = p.held[:0]
		}
		d.recycled = true
	}
	if s, ok := detachFit(&p.free, n); ok {
		d.keep(s)
		p.mu.Unlock()
		return s
	}
	p.mu.Unlock()
	s := allocMaybeHuge(n)
	p.mu.Lock()
	d.keep(s)
	p.mu.Unlock()
	return s
}

func (d *cursor) keep(s []byte) {
	if d.nmine < len(d.mine) {
		d.mine[d.nmine] = s
		d.nmine++
		return
	}
	d.mineExtra = append(d.mineExtra, s)
}

func (d *cursor) finish() {
	if d.nmine == 0 && len(d.mineExtra) == 0 {
		d.recycled = false
		return
	}
	p := d.pool
	if p == nil {
		d.nmine = 0
		d.mineExtra = nil
		d.recycled = false
		return
	}
	p.mu.Lock()
	p.held = append(p.held, d.mine[:d.nmine]...)
	if len(d.mineExtra) > 0 {
		p.held = append(p.held, d.mineExtra...)
	}
	p.mu.Unlock()
	for i := 0; i < d.nmine; i++ {
		d.mine[i] = nil
	}
	d.nmine = 0
	d.mineExtra = nil
	d.recycled = false
}

func detachFit(free *[][]byte, n int) ([]byte, bool) {
	best := -1
	bestCap := int(^uint(0) >> 1)
	bufs := *free
	for i, s := range bufs {
		c := cap(s)
		if c >= n && c < bestCap {
			best = i
			bestCap = c
			if c == n {
				break
			}
		}
	}
	if best < 0 {
		return nil, false
	}
	s := bufs[best]
	bufs[best] = bufs[len(bufs)-1]
	bufs[len(bufs)-1] = nil
	*free = bufs[:len(bufs)-1]
	return s[:n], true
}

func allocPod[T any](d *cursor, n, width int, scratch *[]T, bit uint32) []T {
	if scratch != nil && d.used&bit == 0 && cap(*scratch) >= n {
		d.used |= bit
		*scratch = (*scratch)[:n]
		return *scratch
	}
	var zero T
	if int(unsafe.Sizeof(zero)) == width && (hostLE || width == 1) {
		raw := d.podBytes(n * width)
		return unsafe.Slice((*T)(unsafe.Pointer(&raw[0])), n)
	}
	return make([]T, n)
}

func listNums[T any](d *cursor, n, width int, scratch *[]T, bit uint32, read func([]T, []byte)) (any, error) {
	if n == 0 {
		return []T{}, nil
	}
	s := allocPod(d, n, width, scratch, bit)
	read(s, d.b[d.i:])
	d.i += n * width
	return s, nil
}

func (d *cursor) i8List(n int) (any, error) {
	var sp *[]int8
	if d.scratch != nil {
		sp = &d.scratch.I8
	}
	return listNums(d, n, 1, sp, useI8, readI8s)
}
func (d *cursor) u8List(n int) (any, error) {
	var sp *[]byte
	if d.scratch != nil {
		sp = &d.scratch.U8
	}
	return listNums(d, n, 1, sp, useU8, readU8s)
}
func (d *cursor) i16List(n int) (any, error) {
	var sp *[]int16
	if d.scratch != nil {
		sp = &d.scratch.I16
	}
	return listNums(d, n, 2, sp, useI16, readI16s)
}
func (d *cursor) u16List(n int) (any, error) {
	var sp *[]uint16
	if d.scratch != nil {
		sp = &d.scratch.U16
	}
	return listNums(d, n, 2, sp, useU16, readU16s)
}
func (d *cursor) i32List(n int) (any, error) {
	var sp *[]int32
	if d.scratch != nil {
		sp = &d.scratch.I32
	}
	return listNums(d, n, 4, sp, useI32, readI32s)
}
func (d *cursor) u32List(n int) (any, error) {
	var sp *[]uint32
	if d.scratch != nil {
		sp = &d.scratch.U32
	}
	return listNums(d, n, 4, sp, useU32, readU32s)
}
func (d *cursor) i64List(n int) (any, error) {
	var sp *[]int64
	if d.scratch != nil {
		sp = &d.scratch.I64
	}
	return listNums(d, n, 8, sp, useI64, readI64s)
}
func (d *cursor) u64List(n int) (any, error) {
	var sp *[]uint64
	if d.scratch != nil {
		sp = &d.scratch.U64
	}
	return listNums(d, n, 8, sp, useU64, readU64s)
}
func (d *cursor) intList(n int) (any, error) {
	var sp *[]int
	if d.scratch != nil {
		sp = &d.scratch.Int
	}
	return listNums(d, n, 8, sp, useInt, readInts)
}
func (d *cursor) uintList(n int) (any, error) {
	var sp *[]uint
	if d.scratch != nil {
		sp = &d.scratch.Uint
	}
	return listNums(d, n, 8, sp, useUint, readUints)
}
func (d *cursor) f32List(n int) (any, error) {
	var sp *[]float32
	if d.scratch != nil {
		sp = &d.scratch.F32
	}
	return listNums(d, n, 4, sp, useF32, readF32s)
}
func (d *cursor) f64List(n int) (any, error) {
	var sp *[]float64
	if d.scratch != nil {
		sp = &d.scratch.F64
	}
	return listNums(d, n, 8, sp, useF64, readF64s)
}
