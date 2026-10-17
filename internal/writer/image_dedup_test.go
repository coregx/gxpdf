package writer

import "testing"

func TestDigestImage_EveryFieldCounts(t *testing.T) {
	base := ImageData{
		Data: []byte{1, 2, 3}, AlphaMask: []byte{4},
		Width: 3, Height: 1, ColorSpace: "DeviceRGB", Format: "png", BitsPerComponent: 8,
	}
	same := base
	same.Data = []byte{1, 2, 3} // equal content, different backing array
	if digestImage(&base) != digestImage(&same) {
		t.Fatal("equal content hashed differently")
	}

	variants := map[string]func(*ImageData){
		"data":        func(d *ImageData) { d.Data = []byte{1, 2, 4} },
		"alpha mask":  func(d *ImageData) { d.AlphaMask = nil },
		"width":       func(d *ImageData) { d.Width = 1; d.Height = 3 },
		"color space": func(d *ImageData) { d.ColorSpace = "DeviceGray" },
		"format":      func(d *ImageData) { d.Format = "jpeg" },
		"bpc":         func(d *ImageData) { d.BitsPerComponent = 16 },
		// The same bytes split differently between Data and AlphaMask.
		"data/mask split": func(d *ImageData) { d.Data = []byte{1, 2}; d.AlphaMask = []byte{3, 4} },
	}
	for name, change := range variants {
		v := base
		change(&v)
		if digestImage(&base) == digestImage(&v) {
			t.Errorf("%s change not reflected in the digest", name)
		}
	}
}

func TestWrittenImage_ReusesByPointerAndContent(t *testing.T) {
	w := &PdfWriter{}
	a := &ImageData{Data: []byte{9}, Width: 1, Height: 1, ColorSpace: "DeviceGray", Format: "png", BitsPerComponent: 8}
	_, key, ok := w.writtenImage(a)
	if ok {
		t.Fatal("empty writer reports an image")
	}
	if key != digestImage(a) {
		t.Fatal("miss does not return the digest")
	}
	w.rememberImage(a, key, 7)
	if n, _, ok := w.writtenImage(a); !ok || n != 7 {
		t.Errorf("same pointer: %d %v, want 7 true", n, ok)
	}
	b := *a
	if n, _, ok := w.writtenImage(&b); !ok || n != 7 {
		t.Errorf("same content: %d %v, want 7 true", n, ok)
	}
	c := *a
	c.Data = []byte{8}
	if _, _, ok := w.writtenImage(&c); ok {
		t.Error("different content reported as written")
	}
}
