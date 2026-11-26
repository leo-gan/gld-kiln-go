package kiln

import (
	"encoding/binary"
	"math"
	"unicode/utf8"
	"unsafe"
)

const stringSlabMax = 256 * 1024

// utf8Valid checks UTF-8 with an 8-byte SWAR ASCII fast path.
func utf8Valid(b []byte) bool {
	i := 0
	n := len(b)
	for i+8 <= n {
		if binary.LittleEndian.Uint64(b[i:])&0x8080808080808080 != 0 {
			return utf8.Valid(b[i:])
		}
		i += 8
	}
	for i < n {
		if b[i] >= 0x80 {
			return utf8.Valid(b[i:])
		}
		i++
	}
	return true
}

// Writer encodes positional Kiln wire fields directly into a reusable byte buffer.
type Writer struct {
	buf []byte
	pos int
	cap uint64
}

// NewWriter returns a Writer with at least capacity bytes preallocated.
func NewWriter(capacity int) *Writer {
	if capacity < 64 {
		capacity = 64
	}
	return &Writer{
		buf: allocBytes(capacity),
		cap: DefaultCap,
	}
}

// SetCap sets the maximum allowed element or byte count per string or list.
func (w *Writer) SetCap(c uint64) {
	if c == 0 {
		w.cap = DefaultCap
		return
	}
	w.cap = c
}

// Reset clears the written position and ensures at least need bytes of capacity.
func (w *Writer) Reset(need int) {
	w.pos = 0
	if w.cap == 0 {
		w.cap = DefaultCap
	}
	if need > len(w.buf) {
		if need <= cap(w.buf) {
			w.buf = w.buf[:cap(w.buf)]
		} else {
			next := cap(w.buf) * 2
			if next < need {
				next = need
			}
			if next < 64 {
				next = 64
			}
			w.buf = allocBytes(next)
		}
	}
}

// Bytes returns the encoded slice written since the last Reset.
func (w *Writer) Bytes() []byte {
	return w.buf[:w.pos]
}

func (w *Writer) grow(n int) {
	need := w.pos + n
	if need <= cap(w.buf) {
		w.buf = w.buf[:cap(w.buf)]
		return
	}
	next := cap(w.buf) * 2
	if next < need {
		next = need
	}
	if next < 64 {
		next = 64
	}
	out := allocBytes(next)
	copy(out, w.buf[:w.pos])
	w.buf = out
}

func (w *Writer) ensure(n int) {
	if w.pos+n > len(w.buf) {
		w.grow(n)
	}
}

func (w *Writer) PutBool(v bool) {
	w.ensure(1)
	if v {
		w.buf[w.pos] = 1
	} else {
		w.buf[w.pos] = 0
	}
	w.pos++
}

func (w *Writer) PutU32(v uint32) {
	w.ensure(4)
	binary.LittleEndian.PutUint32(w.buf[w.pos:], v)
	w.pos += 4
}

func (w *Writer) PutI32(v int32) { w.PutU32(uint32(v)) }

func (w *Writer) PutU64(v uint64) {
	w.ensure(8)
	binary.LittleEndian.PutUint64(w.buf[w.pos:], v)
	w.pos += 8
}

func (w *Writer) PutI64(v int64)   { w.PutU64(uint64(v)) }
func (w *Writer) PutF32(v float32) { w.PutU32(math.Float32bits(v)) }
func (w *Writer) PutF64(v float64) { w.PutU64(math.Float64bits(v)) }

func (w *Writer) PutCount(n int) error {
	limit := w.cap
	if limit == 0 {
		limit = DefaultCap
	}
	if n < 0 || uint64(n) > limit || uint64(n) > math.MaxUint32 {
		return Fail("LIMIT")
	}
	w.PutU32(uint32(n))
	return nil
}

func (w *Writer) PutString(s string) error {
	n := len(s)
	limit := w.cap
	if limit == 0 {
		limit = DefaultCap
	}
	if uint64(n) > limit || uint64(n) > math.MaxUint32 {
		return Fail("LIMIT")
	}
	w.ensure(4 + n)
	binary.LittleEndian.PutUint32(w.buf[w.pos:], uint32(n))
	w.pos += 4
	if n > 0 {
		copy(w.buf[w.pos:], s)
		w.pos += n
	}
	return nil
}

func (w *Writer) PutF64s(xs []float64) error {
	if err := w.PutCount(len(xs)); err != nil {
		return err
	}
	if len(xs) == 0 {
		return nil
	}
	nbytes := len(xs) * 8
	w.ensure(nbytes)
	if hostLE {
		src := unsafe.Slice((*byte)(unsafe.Pointer(&xs[0])), nbytes)
		copy(w.buf[w.pos:], src)
		w.pos += nbytes
		return nil
	}
	for _, x := range xs {
		binary.LittleEndian.PutUint64(w.buf[w.pos:], math.Float64bits(x))
		w.pos += 8
	}
	return nil
}

func (w *Writer) PutF32s(xs []float32) error {
	if err := w.PutCount(len(xs)); err != nil {
		return err
	}
	if len(xs) == 0 {
		return nil
	}
	nbytes := len(xs) * 4
	w.ensure(nbytes)
	if hostLE {
		src := unsafe.Slice((*byte)(unsafe.Pointer(&xs[0])), nbytes)
		copy(w.buf[w.pos:], src)
		w.pos += nbytes
		return nil
	}
	for _, x := range xs {
		binary.LittleEndian.PutUint32(w.buf[w.pos:], math.Float32bits(x))
		w.pos += 4
	}
	return nil
}

func (w *Writer) PutI64s(xs []int64) error {
	if err := w.PutCount(len(xs)); err != nil {
		return err
	}
	if len(xs) == 0 {
		return nil
	}
	nbytes := len(xs) * 8
	w.ensure(nbytes)
	if hostLE {
		src := unsafe.Slice((*byte)(unsafe.Pointer(&xs[0])), nbytes)
		copy(w.buf[w.pos:], src)
		w.pos += nbytes
		return nil
	}
	for _, x := range xs {
		binary.LittleEndian.PutUint64(w.buf[w.pos:], uint64(x))
		w.pos += 8
	}
	return nil
}

// Reader decodes positional Kiln wire fields directly from a byte slice.
type Reader struct {
	buf     []byte
	pos     int
	cap     uint64
	trusted bool
	strSlab string
	hasSlab bool
}

// NewReader creates a checked positional Reader over buf.
func NewReader(buf []byte, opt Options) Reader {
	c := opt.Cap
	if c == 0 {
		c = DefaultCap
	}
	return Reader{
		buf:     buf,
		cap:     uint64(c),
		trusted: opt.Trusted,
	}
}

func (r *Reader) Bool() (bool, error) {
	if r.pos >= len(r.buf) {
		return false, Fail("EOF")
	}
	b := r.buf[r.pos]
	r.pos++
	if b > 1 {
		return false, Fail("TYPE")
	}
	return b == 1, nil
}

func (r *Reader) U32() (uint32, error) {
	if len(r.buf)-r.pos < 4 {
		return 0, Fail("EOF")
	}
	v := binary.LittleEndian.Uint32(r.buf[r.pos:])
	r.pos += 4
	return v, nil
}

func (r *Reader) I32() (int32, error) {
	v, err := r.U32()
	return int32(v), err
}

func (r *Reader) U64() (uint64, error) {
	if len(r.buf)-r.pos < 8 {
		return 0, Fail("EOF")
	}
	v := binary.LittleEndian.Uint64(r.buf[r.pos:])
	r.pos += 8
	return v, nil
}

func (r *Reader) I64() (int64, error) {
	v, err := r.U64()
	return int64(v), err
}

func (r *Reader) F32() (float32, error) {
	v, err := r.U32()
	return math.Float32frombits(v), err
}

func (r *Reader) F64() (float64, error) {
	v, err := r.U64()
	return math.Float64frombits(v), err
}

func (r *Reader) Count(each int) (int, error) {
	n, err := r.U32()
	if err != nil {
		return 0, err
	}
	if uint64(n) > r.cap {
		return 0, Fail("LIMIT")
	}
	if each > 0 {
		rem := uint64(len(r.buf) - r.pos)
		if uint64(n)*uint64(each) > rem {
			return 0, Fail("EOF")
		}
	}
	return int(n), nil
}

func (r *Reader) String() (string, error) {
	n, err := r.Count(1)
	if err != nil {
		return "", err
	}
	if n == 0 {
		return "", nil
	}
	start := r.pos
	end := start + n
	if !r.trusted && !utf8Valid(r.buf[start:end]) {
		return "", Fail("UTF8")
	}
	r.pos = end
	if len(r.buf) <= stringSlabMax {
		if !r.hasSlab {
			r.strSlab = string(r.buf)
			r.hasSlab = true
		}
		return r.strSlab[start:end], nil
	}
	return string(r.buf[start:end]), nil
}

func (r *Reader) F64s() ([]float64, error) {
	n, err := r.Count(8)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return []float64{}, nil
	}
	out := allocSlice[float64](n)
	nbytes := n * 8
	readF64s(out, r.buf[r.pos:r.pos+nbytes])
	r.pos += nbytes
	return out, nil
}

func (r *Reader) F32s() ([]float32, error) {
	n, err := r.Count(4)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return []float32{}, nil
	}
	out := allocSlice[float32](n)
	nbytes := n * 4
	readF32s(out, r.buf[r.pos:r.pos+nbytes])
	r.pos += nbytes
	return out, nil
}

func (r *Reader) I64s() ([]int64, error) {
	n, err := r.Count(8)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return []int64{}, nil
	}
	out := allocSlice[int64](n)
	nbytes := n * 8
	readI64s(out, r.buf[r.pos:r.pos+nbytes])
	r.pos += nbytes
	return out, nil
}

func (r *Reader) Finish() error {
	if r.pos != len(r.buf) {
		return Fail("TRAILING")
	}
	return nil
}
