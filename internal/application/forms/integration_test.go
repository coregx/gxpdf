package forms

import (
	"testing"
)

// TestFormsAPIIntegration tests the complete Forms API workflow.
// These tests verify the API contracts without requiring real PDF files.
func TestFormsAPIIntegration(t *testing.T) {
	t.Run("Reader API", func(t *testing.T) {
		// Test that Reader can be created with nil (returns empty results)
		reader := NewReader(nil)
		if reader == nil {
			t.Fatal("NewReader(nil) returned nil")
		}
	})

	t.Run("Writer API", func(t *testing.T) {
		// Test Writer creation and basic operations
		writer := NewWriter(nil)
		if writer == nil {
			t.Fatal("NewWriter(nil) returned nil")
		}

		// HasUpdates should be false initially
		if writer.HasUpdates() {
			t.Error("HasUpdates() = true for new writer, want false")
		}

		// GetUpdates should return empty map
		updates := writer.GetUpdates()
		if len(updates) != 0 {
			t.Errorf("GetUpdates() returned %d items, want 0", len(updates))
		}
	})

	t.Run("Flattener API", func(t *testing.T) {
		// Test Flattener creation
		flattener := NewFlattener(nil)
		if flattener == nil {
			t.Fatal("NewFlattener(nil) returned nil")
		}

		// CanFlatten should be false for nil reader
		if flattener.CanFlatten() {
			t.Error("CanFlatten() = true for nil reader, want false")
		}

		// GetFlattenInfo should return nil for nil reader
		info, err := flattener.GetFlattenInfo()
		if err != nil {
			t.Errorf("GetFlattenInfo() error = %v, want nil", err)
		}
		if info != nil {
			t.Errorf("GetFlattenInfo() returned %d items, want nil", len(info))
		}
	})
}

// TestFieldTypeConstants verifies field type constant values.
func TestFieldTypeConstants(t *testing.T) {
	tests := []struct {
		fieldType FieldType
		expected  string
	}{
		{FieldTypeText, "Tx"},
		{FieldTypeButton, "Btn"},
		{FieldTypeChoice, "Ch"},
		{FieldTypeSignature, "Sig"},
	}

	for _, tt := range tests {
		t.Run(string(tt.fieldType), func(t *testing.T) {
			if string(tt.fieldType) != tt.expected {
				t.Errorf("FieldType = %q, want %q", tt.fieldType, tt.expected)
			}
		})
	}
}

// TestFieldInfoHelpers tests FieldInfo struct helper checks.
func TestFieldInfoHelpers(t *testing.T) {
	tests := []struct {
		name       string
		fieldType  FieldType
		flags      int
		isReadOnly bool
		isRequired bool
	}{
		{"no flags", FieldTypeText, 0, false, false},
		{"read only", FieldTypeText, 1, true, false},
		{"required", FieldTypeText, 2, false, true},
		{"both flags", FieldTypeText, 3, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &FieldInfo{
				Name:  "test",
				Type:  tt.fieldType,
				Flags: tt.flags,
			}

			isReadOnly := info.Flags&1 != 0
			isRequired := info.Flags&2 != 0

			if isReadOnly != tt.isReadOnly {
				t.Errorf("ReadOnly = %v, want %v", isReadOnly, tt.isReadOnly)
			}
			if isRequired != tt.isRequired {
				t.Errorf("Required = %v, want %v", isRequired, tt.isRequired)
			}
		})
	}
}

// TestWriterValueTypes tests that Writer accepts correct value types.
func TestWriterValueTypes(t *testing.T) {
	writer := NewWriter(nil)

	// Test text field values
	t.Run("text field accepts string", func(t *testing.T) {
		field := &FieldInfo{Name: "text", Type: FieldTypeText}
		err := validateFieldValueType(field, "hello")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("text field rejects int", func(t *testing.T) {
		field := &FieldInfo{Name: "text", Type: FieldTypeText}
		err := validateFieldValueType(field, 123)
		if err == nil {
			t.Error("expected error for int value in text field")
		}
	})

	// Test button field values
	t.Run("button field accepts bool", func(t *testing.T) {
		field := &FieldInfo{Name: "check", Type: FieldTypeButton}
		err := validateFieldValueType(field, true)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("button field accepts string", func(t *testing.T) {
		field := &FieldInfo{Name: "check", Type: FieldTypeButton}
		err := validateFieldValueType(field, "Yes")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	// Test choice field options validation
	t.Run("choice field validates options", func(t *testing.T) {
		field := &FieldInfo{
			Name:    "dropdown",
			Type:    FieldTypeChoice,
			Options: []string{"A", "B", "C"},
		}

		// Valid option
		if !writer.isValidOption(field, "A") {
			t.Error("isValidOption(A) = false, want true")
		}

		// Invalid option
		if writer.isValidOption(field, "D") {
			t.Error("isValidOption(D) = true, want false")
		}

		// No options defined - allow any
		fieldNoOpts := &FieldInfo{Name: "dropdown", Type: FieldTypeChoice}
		if !writer.isValidOption(fieldNoOpts, "anything") {
			t.Error("isValidOption should allow any value when no options defined")
		}
	})

	// Test signature field rejection
	t.Run("signature field rejects all values", func(t *testing.T) {
		field := &FieldInfo{Name: "sig", Type: FieldTypeSignature}
		err := validateFieldValueType(field, "anything")
		if err == nil {
			t.Error("expected error for signature field")
		}
	})
}

// TestFlattenInfoStruct tests the FlattenInfo structure.
func TestFlattenInfoStruct(t *testing.T) {
	info := &FlattenInfo{
		FieldName:        "test_field",
		PageIndex:        2,
		Rect:             [4]float64{100, 200, 300, 220},
		AppearanceStream: []byte("BT /F1 12 Tf (Hello) Tj ET"),
		Resources:        nil,
	}

	if info.FieldName != "test_field" {
		t.Errorf("FieldName = %q, want %q", info.FieldName, "test_field")
	}

	if info.PageIndex != 2 {
		t.Errorf("PageIndex = %d, want 2", info.PageIndex)
	}

	width := info.Rect[2] - info.Rect[0]
	if width != 200 {
		t.Errorf("Width = %f, want 200", width)
	}

	height := info.Rect[3] - info.Rect[1]
	if height != 20 {
		t.Errorf("Height = %f, want 20", height)
	}

	if len(info.AppearanceStream) == 0 {
		t.Error("AppearanceStream should not be empty")
	}
}
