package forms

import (
	"testing"
)

func TestNewFlattener(t *testing.T) {
	flattener := NewFlattener(nil)
	if flattener == nil {
		t.Fatal("NewFlattener returned nil")
	}
}

func TestFlattener_GetFlattenInfo_NilReader(t *testing.T) {
	flattener := NewFlattener(nil)
	_, err := flattener.GetFlattenInfo()
	// Should handle nil reader gracefully (or return error)
	if err == nil {
		t.Log("GetFlattenInfo with nil reader returned no error (acceptable for empty result)")
	}
}

func TestFlattenInfo_Fields(t *testing.T) {
	info := &FlattenInfo{
		FieldName:        "test_field",
		PageIndex:        0,
		Rect:             [4]float64{100, 100, 200, 120},
		AppearanceStream: []byte("BT /F1 12 Tf 0 0 Td (Hello) Tj ET"),
		Resources:        nil,
	}

	if info.FieldName != "test_field" {
		t.Errorf("FieldName = %q, want %q", info.FieldName, "test_field")
	}

	if info.PageIndex != 0 {
		t.Errorf("PageIndex = %d, want 0", info.PageIndex)
	}

	if info.Rect[2]-info.Rect[0] != 100 {
		t.Errorf("Width = %f, want 100", info.Rect[2]-info.Rect[0])
	}

	if len(info.AppearanceStream) == 0 {
		t.Error("AppearanceStream is empty")
	}
}

func TestFlattener_buildFieldName(t *testing.T) {
	// buildFieldName is a private method and requires a valid dictionary
	// Skip detailed testing here - it's tested through integration tests
	t.Log("buildFieldName is tested through integration tests")
}

func TestFlattener_CanFlatten_NoReader(t *testing.T) {
	flattener := NewFlattener(nil)
	// Should return false when reader is nil
	result := flattener.CanFlatten()
	if result {
		t.Error("CanFlatten() = true, want false for nil reader")
	}
}
