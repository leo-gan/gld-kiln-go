package kiln

import (
	"bytes"
	"encoding/binary"
	"unicode/utf8"
	"unsafe"
)

const (
	headerSize = 64
	descSize   = 128
	flagName   = 0x01
	flagLE     = 0x02
	opaqueMin  = 4096
	opaqueMax  = 32767
)

// Tensor is one directory entry. Data aliases the input buffer on decode.
type Tensor struct {
	Name        string
	DType       uint16
	Shape       []uint64
	Data        []byte
	Layout      string
	BlockSize   uint32
	QuantScheme uint16
	QuantAxis   uint16
	GroupSize   uint32
	ScaleIndex  uint32
	OuterIndex  uint32
	shapeArr    [8]uint64
}

// Directory is one version-2 tensor block.
// A one-tensor directory keeps that tensor inline so decode does not allocate a second slice.
type Directory struct {
	Flags       byte
	Tensors     []Tensor
	BlockLength int
	only        Tensor
}

func dtypeName(code uint16) (string, bool) {
	names := [...]string{
		1: "f64", 2: "f32", 3: "f16", 4: "bf16", 5: "f8e4m3", 6: "f8e5m2",
		7: "bool", 8: "i8", 9: "u8", 10: "i16", 11: "u16", 12: "i32", 13: "u32",
		14: "i64", 15: "u64", 16: "i4", 17: "u4", 18: "e2m1", 19: "e8m0",
	}
	if int(code) < len(names) && names[code] != "" {
		return names[code], true
	}
	return "", false
}

func wholeWidth(dt uint16) (uint64, bool) {
	switch dt {
	case 1, 14, 15:
		return 8, true
	case 2, 12, 13:
		return 4, true
	case 3, 4, 10, 11:
		return 2, true
	case 5, 6, 7, 8, 9, 19:
		return 1, true
	default:
		return 0, false
	}
}

func packedDtype(dt uint16) bool { return dt == 16 || dt == 17 || dt == 18 }

func add64(a, b uint64) (uint64, error) {
	if a > ^uint64(0)-b {
		return 0, Fail("LIMIT")
	}
	return a + b, nil
}

func mul64(a, b uint64) (uint64, error) {
	if a != 0 && b > ^uint64(0)/a {
		return 0, Fail("LIMIT")
	}
	return a * b, nil
}

func align64u(n uint64) uint64 { return (n + 63) &^ 63 }

func nonzero(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return true
		}
	}
	return false
}

type rawTensor struct {
	dtype                  uint16
	rank, reserved         uint8
	nameOff, nameLen       uint16
	layoutOff, layoutLen   uint16
	blockSize              uint32
	quantScheme, quantAxis uint16
	groupSize              uint32
	scaleIndex, outerIndex uint32
	byteOffset, byteLength uint64
	shape                  [8]uint64
	name, layout           []byte
}

func appendTensors(dst []byte, v any) ([]byte, error) {
	var ts []Tensor
	switch x := v.(type) {
	case *Directory:
		if x != nil {
			ts = x.Tensors
		}
	case Directory:
		ts = x.Tensors
	case []Tensor:
		ts = x
	default:
		return nil, Fail("TYPE")
	}
	return AppendTensors(dst, ts)
}

// EncodeTensors writes one canonical version-2 directory.
func EncodeTensors(ts []Tensor) ([]byte, error) {
	return AppendTensors(nil, ts)
}

// AppendTensors appends one canonical directory to dst.
func AppendTensors(dst []byte, ts []Tensor) ([]byte, error) {
	type plan struct {
		nameOff, nameLen     uint16
		layoutOff, layoutLen uint16
		off                  int
	}
	var stack [8]plan
	var plans []plan
	if len(ts) <= len(stack) {
		plans = stack[:len(ts)]
	} else {
		plans = make([]plan, len(ts))
	}
	heapLen := 0
	place := func(text string, off, ln *uint16) error {
		if text == "" {
			*off = 0xFFFF
			*ln = 0
			return nil
		}
		if heapLen > 0xFFFF || len(text) > 0xFFFF {
			return Fail("LIMIT")
		}
		*off = uint16(heapLen)
		*ln = uint16(len(text))
		heapLen += len(text)
		return nil
	}
	for i, t := range ts {
		if len(t.Shape) > 8 {
			return nil, Fail("TYPE")
		}
		if err := place(t.Name, &plans[i].nameOff, &plans[i].nameLen); err != nil {
			return nil, err
		}
	}
	for i, t := range ts {
		if err := place(t.Layout, &plans[i].layoutOff, &plans[i].layoutLen); err != nil {
			return nil, err
		}
	}
	end := 0
	for i := range plans {
		off := (end + 63) &^ 63
		plans[i].off = off
		end = off + len(ts[i].Data)
	}
	dirBytes := len(ts)*descSize + heapLen
	payloadBase := (headerSize + dirBytes + 63) &^ 63
	total := payloadBase + end
	baseOut := len(dst)
	fresh := false
	if cap(dst)-len(dst) < total {
		next := make([]byte, baseOut+total)
		copy(next, dst)
		dst = next
		fresh = true
	} else {
		dst = dst[:baseOut+total]
	}
	buf := dst[baseOut:]
	if !fresh {
		clear(buf)
	}
	copy(buf[:4], "KLNT")
	buf[4] = 2
	flags := byte(flagLE)
	for _, t := range ts {
		if t.Name != "" {
			flags |= flagName
			break
		}
	}
	buf[5] = flags
	binary.LittleEndian.PutUint32(buf[8:], uint32(len(ts)))
	binary.LittleEndian.PutUint32(buf[12:], uint32(heapLen))
	binary.LittleEndian.PutUint64(buf[16:], uint64(end))
	binary.LittleEndian.PutUint64(buf[24:], uint64(dirBytes))
	heapAt := headerSize + len(ts)*descSize
	for i, p := range plans {
		base := headerSize + i*descSize
		t := ts[i]
		if t.QuantScheme == 0 {
			t.QuantAxis = 0xFFFF
			t.GroupSize = 0
			t.ScaleIndex = 0xFFFFFFFF
			t.OuterIndex = 0xFFFFFFFF
		}
		binary.LittleEndian.PutUint16(buf[base:], t.DType)
		buf[base+2] = byte(len(t.Shape))
		binary.LittleEndian.PutUint16(buf[base+4:], p.nameOff)
		binary.LittleEndian.PutUint16(buf[base+6:], p.nameLen)
		binary.LittleEndian.PutUint16(buf[base+8:], p.layoutOff)
		binary.LittleEndian.PutUint16(buf[base+10:], p.layoutLen)
		binary.LittleEndian.PutUint32(buf[base+12:], t.BlockSize)
		binary.LittleEndian.PutUint16(buf[base+16:], t.QuantScheme)
		binary.LittleEndian.PutUint16(buf[base+18:], t.QuantAxis)
		binary.LittleEndian.PutUint32(buf[base+20:], t.GroupSize)
		binary.LittleEndian.PutUint32(buf[base+24:], t.ScaleIndex)
		binary.LittleEndian.PutUint32(buf[base+28:], t.OuterIndex)
		binary.LittleEndian.PutUint64(buf[base+32:], uint64(p.off))
		binary.LittleEndian.PutUint64(buf[base+40:], uint64(len(t.Data)))
		for dim := 0; dim < 8; dim++ {
			var sv uint64
			if dim < len(t.Shape) {
				sv = t.Shape[dim]
			}
			binary.LittleEndian.PutUint64(buf[base+48+8*dim:], sv)
		}
		if p.nameLen > 0 {
			copy(buf[heapAt+int(p.nameOff):], t.Name)
		}
		if p.layoutLen > 0 {
			copy(buf[heapAt+int(p.layoutOff):], t.Layout)
		}
		copy(buf[payloadBase+p.off:], t.Data)
	}
	return dst, nil
}

func (d *cursor) tensors() (any, error) {
	dir, n, err := decodeDirectory(d.b[d.i:], d.capn, d.trusted)
	if err != nil {
		return nil, err
	}
	d.i += n
	return dir, nil
}

func decodeDirectory(buf []byte, capn int, trusted bool) (*Directory, int, error) {
	if capn <= 0 {
		capn = DefaultCap
	}
	if len(buf) < 4 {
		return nil, 0, Fail("EOF")
	}
	if buf[0] != 'K' || buf[1] != 'L' || buf[2] != 'N' || buf[3] != 'T' {
		return nil, 0, Fail("FRAME")
	}
	if len(buf) < headerSize {
		return nil, 0, Fail("EOF")
	}
	if buf[4] != 2 {
		return nil, 0, Fail("FRAME")
	}
	flags := buf[5]
	if flags&^0x03 != 0 || flags&flagLE == 0 {
		return nil, 0, Fail("FRAME")
	}
	if binary.LittleEndian.Uint16(buf[6:]) != 0 || nonzero(buf[32:headerSize]) {
		return nil, 0, Fail("FRAME")
	}
	count := binary.LittleEndian.Uint32(buf[8:])
	nameBytes := binary.LittleEndian.Uint32(buf[12:])
	payloadBytes := binary.LittleEndian.Uint64(buf[16:])
	directoryBytes := binary.LittleEndian.Uint64(buf[24:])
	if uint64(count) > uint64(capn) || uint64(nameBytes) > uint64(capn) {
		return nil, 0, Fail("LIMIT")
	}
	prod, err := mul64(uint64(count), descSize)
	if err != nil {
		return nil, 0, err
	}
	expectDir, err := add64(prod, uint64(nameBytes))
	if err != nil {
		return nil, 0, err
	}
	if directoryBytes != expectDir {
		return nil, 0, Fail("TYPE")
	}
	directoryEnd, err := add64(headerSize, directoryBytes)
	if err != nil {
		return nil, 0, err
	}
	if directoryEnd > uint64(len(buf)) {
		return nil, 0, Fail("EOF")
	}
	payloadBase := align64u(directoryEnd)
	blockEnd, err := add64(payloadBase, payloadBytes)
	if err != nil {
		return nil, 0, err
	}
	if blockEnd > uint64(len(buf)) {
		return nil, 0, Fail("EOF")
	}
	if nonzero(buf[int(directoryEnd):int(payloadBase)]) {
		return nil, 0, Fail("TYPE")
	}
	if count == 0 && nameBytes == 0 && payloadBytes == 0 {
		if flags&flagName != 0 {
			return nil, 0, Fail("TYPE")
		}
		return &Directory{Flags: flags, BlockLength: int(blockEnd)}, int(blockEnd), nil
	}
	// The directory lies inside buf. A small count stays on the stack.
	var rawStack [4]rawTensor
	var raw []rawTensor
	if count <= uint32(len(rawStack)) {
		raw = rawStack[:count]
	} else {
		raw = make([]rawTensor, count)
	}
	for index := uint32(0); index < count; index++ {
		base := headerSize + int(index)*descSize
		if nonzero(buf[base+112 : base+descSize]) {
			return nil, 0, Fail("FRAME")
		}
		it := &raw[index]
		it.dtype = binary.LittleEndian.Uint16(buf[base:])
		it.rank = buf[base+2]
		it.reserved = buf[base+3]
		it.nameOff = binary.LittleEndian.Uint16(buf[base+4:])
		it.nameLen = binary.LittleEndian.Uint16(buf[base+6:])
		it.layoutOff = binary.LittleEndian.Uint16(buf[base+8:])
		it.layoutLen = binary.LittleEndian.Uint16(buf[base+10:])
		it.blockSize = binary.LittleEndian.Uint32(buf[base+12:])
		it.quantScheme = binary.LittleEndian.Uint16(buf[base+16:])
		it.quantAxis = binary.LittleEndian.Uint16(buf[base+18:])
		it.groupSize = binary.LittleEndian.Uint32(buf[base+20:])
		it.scaleIndex = binary.LittleEndian.Uint32(buf[base+24:])
		it.outerIndex = binary.LittleEndian.Uint32(buf[base+28:])
		it.byteOffset = binary.LittleEndian.Uint64(buf[base+32:])
		it.byteLength = binary.LittleEndian.Uint64(buf[base+40:])
		for dim := 0; dim < 8; dim++ {
			it.shape[dim] = binary.LittleEndian.Uint64(buf[base+48+8*dim:])
		}
	}
	heap := buf[headerSize+int(count)*descSize : int(directoryEnd)]
	namesFlag := flags&flagName != 0
	hasName := false
	for i := range raw {
		if raw[i].nameLen != 0 {
			hasName = true
			break
		}
	}
	if namesFlag != hasName {
		return nil, 0, Fail("TYPE")
	}
	if namesFlag {
		for i := range raw {
			if raw[i].nameLen == 0 {
				return nil, 0, Fail("TYPE")
			}
		}
	}
	var parts, layouts [][]byte
	var seen map[string]struct{}
	for i := range raw {
		it := &raw[i]
		if it.reserved != 0 {
			return nil, 0, Fail("FRAME")
		}
		if it.nameLen == 0 {
			if it.nameOff != 0xFFFF {
				return nil, 0, Fail("TYPE")
			}
		} else {
			if it.nameOff == 0xFFFF {
				return nil, 0, Fail("TYPE")
			}
			end, err := add64(uint64(it.nameOff), uint64(it.nameLen))
			if err != nil {
				return nil, 0, err
			}
			if end > uint64(nameBytes) {
				return nil, 0, Fail("TYPE")
			}
			name := heap[int(it.nameOff) : int(it.nameOff)+int(it.nameLen)]
			if !trusted && !utf8.Valid(name) {
				return nil, 0, Fail("UTF8")
			}
			key := string(name)
			if seen == nil {
				seen = make(map[string]struct{}, len(raw))
			}
			if _, ok := seen[key]; ok {
				return nil, 0, Fail("TYPE")
			}
			seen[key] = struct{}{}
			parts = append(parts, name)
			it.name = name
		}
		if it.layoutLen == 0 {
			if it.layoutOff != 0xFFFF {
				return nil, 0, Fail("TYPE")
			}
		} else {
			end, err := add64(uint64(it.layoutOff), uint64(it.layoutLen))
			if err != nil {
				return nil, 0, err
			}
			if end > uint64(nameBytes) {
				return nil, 0, Fail("TYPE")
			}
			layout := heap[int(it.layoutOff) : int(it.layoutOff)+int(it.layoutLen)]
			if !trusted && !utf8.Valid(layout) {
				return nil, 0, Fail("UTF8")
			}
			layouts = append(layouts, layout)
			it.layout = layout
		}
	}
	if !heapExact(heap, parts, layouts) {
		return nil, 0, Fail("TYPE")
	}
	type span struct{ a, b uint64 }
	var rangeStack [4]span
	var ranges []span
	if len(raw) <= len(rangeStack) {
		ranges = rangeStack[:len(raw)]
	} else {
		ranges = make([]span, len(raw))
	}
	for i := range raw {
		it := &raw[i]
		itemEnd, err := add64(it.byteOffset, it.byteLength)
		if err != nil {
			return nil, 0, err
		}
		if itemEnd > payloadBytes {
			return nil, 0, Fail("TYPE")
		}
		ranges[i] = span{it.byteOffset, itemEnd}
		if err := checkTensor(it); err != nil {
			return nil, 0, err
		}
	}
	for left := 0; left < len(ranges); left++ {
		for right := left + 1; right < len(ranges); right++ {
			a0, a1 := ranges[left].a, ranges[left].b
			b0, b1 := ranges[right].a, ranges[right].b
			if a0 == a1 || b0 == b1 {
				continue
			}
			if a0 < b1 && b0 < a1 {
				return nil, 0, Fail("TYPE")
			}
		}
	}
	end := uint64(0)
	for i := range raw {
		if raw[i].byteOffset != align64u(end) {
			return nil, 0, Fail("TYPE")
		}
		end = ranges[i].b
	}
	if end != payloadBytes {
		return nil, 0, Fail("TYPE")
	}
	payload := buf[int(payloadBase):int(blockEnd)]
	for i := range raw {
		it := &raw[i]
		region := payload[int(it.byteOffset) : int(it.byteOffset)+int(it.byteLength)]
		if packedDtype(it.dtype) {
			cnt, err := elementCount(it.rank, it.shape[:])
			if err != nil {
				return nil, 0, err
			}
			if cnt%2 == 1 && len(region) > 0 && region[len(region)-1]>>4 != 0 {
				return nil, 0, Fail("TYPE")
			}
		}
		if it.dtype == 7 {
			for _, b := range region {
				if b > 1 {
					return nil, 0, Fail("TYPE")
				}
			}
		}
	}
	cursorEnd := uint64(0)
	for i := range raw {
		off := raw[i].byteOffset
		if nonzero(payload[int(cursorEnd):int(off)]) {
			return nil, 0, Fail("TYPE")
		}
		cursorEnd = ranges[i].b
	}
	if !trusted {
		if err := checkQuant(raw); err != nil {
			return nil, 0, err
		}
	}
	if len(raw) == 1 {
		out := &Directory{Flags: flags, BlockLength: int(blockEnd)}
		fillTensor(&out.only, &raw[0], payload)
		out.Tensors = unsafe.Slice(&out.only, 1)
		return out, int(blockEnd), nil
	}
	out := &Directory{Flags: flags, Tensors: make([]Tensor, len(raw)), BlockLength: int(blockEnd)}
	for i := range raw {
		fillTensor(&out.Tensors[i], &raw[i], payload)
	}
	return out, int(blockEnd), nil
}

func heapExact(heap []byte, parts, layouts [][]byte) bool {
	i := 0
	for _, p := range parts {
		if i+len(p) > len(heap) || !bytes.Equal(heap[i:i+len(p)], p) {
			return false
		}
		i += len(p)
	}
	for _, p := range layouts {
		if i+len(p) > len(heap) || !bytes.Equal(heap[i:i+len(p)], p) {
			return false
		}
		i += len(p)
	}
	return i == len(heap)
}

func fillTensor(dst *Tensor, it *rawTensor, payload []byte) {
	dst.shapeArr = it.shape
	dst.Shape = dst.shapeArr[:it.rank]
	if it.byteLength > 0 {
		off := int(it.byteOffset)
		dst.Data = payload[off : off+int(it.byteLength)]
	} else {
		dst.Data = nil
	}
	dst.Name = string(it.name)
	dst.Layout = string(it.layout)
	dst.DType = it.dtype
	dst.BlockSize = it.blockSize
	dst.QuantScheme = it.quantScheme
	dst.QuantAxis = it.quantAxis
	dst.GroupSize = it.groupSize
	dst.ScaleIndex = it.scaleIndex
	dst.OuterIndex = it.outerIndex
}

func elementCount(rank uint8, shape []uint64) (uint64, error) {
	if rank == 0 {
		return 1, nil
	}
	count := uint64(1)
	for dim := uint8(0); dim < rank; dim++ {
		var err error
		count, err = mul64(count, shape[dim])
		if err != nil {
			return 0, err
		}
	}
	return count, nil
}

func expectedBytes(dt uint16, count uint64) (uint64, error) {
	if w, ok := wholeWidth(dt); ok {
		return mul64(count, w)
	}
	if packedDtype(dt) {
		q := count / 2
		if count%2 == 1 {
			if q == ^uint64(0) {
				return 0, Fail("LIMIT")
			}
			q++
		}
		return q, nil
	}
	return 0, Fail("TYPE")
}

func checkTensor(it *rawTensor) error {
	if it.rank > 8 {
		return Fail("TYPE")
	}
	for dim := int(it.rank); dim < 8; dim++ {
		if it.shape[dim] != 0 {
			return Fail("TYPE")
		}
	}
	dt := it.dtype
	if dt == 0 || (dt >= 20 && dt < opaqueMin) || dt > opaqueMax {
		return Fail("TYPE")
	}
	count, err := elementCount(it.rank, it.shape[:])
	if err != nil {
		return err
	}
	if dt >= opaqueMin {
		if len(it.layout) == 0 || it.blockSize == 0 {
			return Fail("TYPE")
		}
		if uint64(it.blockSize) == 0 || it.byteLength%uint64(it.blockSize) != 0 {
			return Fail("TYPE")
		}
		return nil
	}
	if len(it.layout) != 0 || it.blockSize != 0 {
		return Fail("TYPE")
	}
	expect, err := expectedBytes(dt, count)
	if err != nil {
		return err
	}
	if it.byteLength != expect {
		return Fail("TYPE")
	}
	return nil
}

func checkQuant(ts []rawTensor) error {
	for index := range ts {
		item := &ts[index]
		switch item.quantScheme {
		case 0:
			if item.groupSize != 0 || item.quantAxis != 0xFFFF || item.scaleIndex != 0xFFFFFFFF || item.outerIndex != 0xFFFFFFFF {
				return Fail("TYPE")
			}
		case 1, 2:
			if item.rank == 0 || int(item.quantAxis) >= int(item.rank) || item.groupSize == 0 {
				return Fail("TYPE")
			}
			scale := item.scaleIndex
			if scale == uint32(index) || int(scale) >= len(ts) {
				return Fail("TYPE")
			}
			other := &ts[scale]
			if other.quantScheme != 0 {
				return Fail("TYPE")
			}
			groups, err := ceilDiv(item.shape[item.quantAxis], uint64(item.groupSize))
			if err != nil {
				return err
			}
			otherDims := uint64(1)
			for dim := uint8(0); dim < item.rank; dim++ {
				if uint16(dim) == item.quantAxis {
					continue
				}
				otherDims, err = mul64(otherDims, item.shape[dim])
				if err != nil {
					return err
				}
			}
			expect, err := mul64(groups, otherDims)
			if err != nil {
				return err
			}
			scaleCount, err := elementCount(other.rank, other.shape[:])
			if err != nil {
				return err
			}
			if scaleCount != expect {
				return Fail("TYPE")
			}
			if item.quantScheme == 1 {
				if item.outerIndex != 0xFFFFFFFF {
					return Fail("TYPE")
				}
				continue
			}
			outer := item.outerIndex
			if outer == uint32(index) || outer == scale || int(outer) >= len(ts) {
				return Fail("TYPE")
			}
			o := &ts[outer]
			oc, err := elementCount(o.rank, o.shape[:])
			if err != nil {
				return err
			}
			if o.quantScheme != 0 || o.dtype != 2 || oc != 1 {
				return Fail("TYPE")
			}
		default:
			return Fail("TYPE")
		}
	}
	return nil
}

func ceilDiv(n, d uint64) (uint64, error) {
	if d == 0 {
		return 0, Fail("TYPE")
	}
	q := n / d
	if n%d != 0 {
		nq, err := add64(q, 1)
		if err != nil {
			return 0, err
		}
		return nq, nil
	}
	return q, nil
}
