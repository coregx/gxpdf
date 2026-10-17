package writer

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"
)

// imageDigest identifies image content: two ImageData values with the same
// digest produce identical image XObjects (and SMasks).
type imageDigest [sha256.Size]byte

// digestImage hashes every field that ends up in the image XObject or its
// SMask. Variable-length fields are length-prefixed so that different splits
// of the same bytes between Data and AlphaMask cannot collide.
func digestImage(img *ImageData) imageDigest {
	h := sha256.New()
	writeField(h, []byte(img.Format))
	writeField(h, []byte(img.ColorSpace))
	var dims [24]byte
	binary.BigEndian.PutUint64(dims[0:], uint64(img.Width))
	binary.BigEndian.PutUint64(dims[8:], uint64(img.Height))
	binary.BigEndian.PutUint64(dims[16:], uint64(img.BitsPerComponent))
	h.Write(dims[:])
	writeField(h, img.Data)
	writeField(h, img.AlphaMask)
	var d imageDigest
	h.Sum(d[:0])
	return d
}

func writeField(h hash.Hash, b []byte) {
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], uint64(len(b)))
	h.Write(n[:])
	h.Write(b)
}

// writtenImage returns the object number of an image XObject already created
// for img or for an image with the same content. When there is none, it
// returns img's digest for rememberImage (hashed once per new ImageData).
func (w *PdfWriter) writtenImage(img *ImageData) (objNum int, key imageDigest, ok bool) {
	if objNum, ok := w.imagesByData[img]; ok {
		return objNum, key, true
	}
	key = digestImage(img)
	if objNum, ok := w.imagesByDigest[key]; ok {
		w.imagesByData[img] = objNum
		return objNum, key, true
	}
	return 0, key, false
}

// rememberImage records the image XObject created for img, whose digest is key.
func (w *PdfWriter) rememberImage(img *ImageData, key imageDigest, objNum int) {
	if w.imagesByData == nil {
		w.imagesByData = make(map[*ImageData]int)
		w.imagesByDigest = make(map[imageDigest]int)
	}
	w.imagesByData[img] = objNum
	w.imagesByDigest[key] = objNum
}
