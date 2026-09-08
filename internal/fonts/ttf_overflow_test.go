package fonts

import (
	"encoding/binary"
	"testing"
)

// A table entry whose Offset+Length overflows uint32 must be rejected, not
// panic on an out-of-bounds slice. The bounds check adds two uint32 values,
// so a wrap-around (e.g. Offset=0xFFFFFFFF, Length=2) passes the check while
// the subsequent slice data[Offset:Offset+Length] is out of range.
func TestLoadTTFFromBytes_TableOffsetLengthOverflow(t *testing.T) {
	var b []byte
	put32 := func(v uint32) {
		tmp := make([]byte, 4)
		binary.BigEndian.PutUint32(tmp, v)
		b = append(b, tmp...)
	}
	put16 := func(v uint16) {
		tmp := make([]byte, 2)
		binary.BigEndian.PutUint16(tmp, v)
		b = append(b, tmp...)
	}

	put32(0x00010000) // sfnt version (TrueType)
	put16(1)          // numTables
	put16(0)          // searchRange
	put16(0)          // entrySelector
	put16(0)          // rangeShift

	// One table directory entry.
	b = append(b, []byte("cmap")...) // tag
	put32(0)                         // checksum
	put32(0xFFFFFFFF)                // offset
	put32(2)                         // length -> offset+length wraps to 1

	// Some trailing data so len(data) > 1.
	b = append(b, make([]byte, 32)...)

	// Must not panic.
	_, _ = LoadTTFFromBytes(b)
}
