package creator

import (
	"testing"
)

func TestNewSurface(t *testing.T) {
	page := &Page{}
	surface := NewSurface(page)

	if surface == nil {
		t.Fatal("NewSurface returned nil")
	}

	if surface.page != page {
		t.Error("Surface.page not set correctly")
	}

	if surface.StackDepth() != 0 {
		t.Errorf("StackDepth = %d, want 0", surface.StackDepth())
	}

	// Check default state
	if surface.CurrentOpacity() != 1.0 {
		t.Errorf("CurrentOpacity = %f, want 1.0", surface.CurrentOpacity())
	}

	if surface.CurrentBlendMode() != BlendModeNormal {
		t.Errorf("CurrentBlendMode = %v, want Normal", surface.CurrentBlendMode())
	}

	if surface.CurrentClipPath() != nil {
		t.Error("CurrentClipPath should be nil by default")
	}
}

func TestPageSurface(t *testing.T) {
	page := &Page{}
	surface := page.Surface()

	if surface == nil {
		t.Fatal("page.Surface() returned nil")
	}

	if surface.page != page {
		t.Error("Surface.page not set correctly")
	}
}

func TestPushPopTransform(t *testing.T) {
	surface := NewSurface(&Page{})

	// Initial transform should be identity
	initialTransform := surface.CurrentTransform()
	if initialTransform != Identity() {
		t.Error("Initial transform is not identity")
	}

	// Push a translation
	surface.PushTransform(Translate(100, 200))

	if surface.StackDepth() != 1 {
		t.Errorf("StackDepth = %d, want 1", surface.StackDepth())
	}

	// Transform should have changed
	currentTransform := surface.CurrentTransform()
	if currentTransform == initialTransform {
		t.Error("Transform did not change after PushTransform")
	}

	// Pop should restore original
	surface.Pop()

	if surface.StackDepth() != 0 {
		t.Errorf("StackDepth = %d, want 0 after Pop", surface.StackDepth())
	}

	restoredTransform := surface.CurrentTransform()
	if restoredTransform != initialTransform {
		t.Error("Transform not restored after Pop")
	}
}

func TestPushPopOpacity(t *testing.T) {
	surface := NewSurface(&Page{})

	// Initial opacity should be 1.0
	if surface.CurrentOpacity() != 1.0 {
		t.Errorf("Initial opacity = %f, want 1.0", surface.CurrentOpacity())
	}

	// Push opacity
	err := surface.PushOpacity(0.5)
	if err != nil {
		t.Fatalf("PushOpacity failed: %v", err)
	}

	if surface.CurrentOpacity() != 0.5 {
		t.Errorf("CurrentOpacity = %f, want 0.5", surface.CurrentOpacity())
	}

	// Push another opacity (should be multiplicative)
	err = surface.PushOpacity(0.8)
	if err != nil {
		t.Fatalf("PushOpacity failed: %v", err)
	}

	expected := 0.5 * 0.8
	if surface.CurrentOpacity() != expected {
		t.Errorf("CurrentOpacity = %f, want %f (0.5 * 0.8)", surface.CurrentOpacity(), expected)
	}

	// Pop once
	surface.Pop()
	if surface.CurrentOpacity() != 0.5 {
		t.Errorf("CurrentOpacity = %f, want 0.5 after first Pop", surface.CurrentOpacity())
	}

	// Pop again
	surface.Pop()
	if surface.CurrentOpacity() != 1.0 {
		t.Errorf("CurrentOpacity = %f, want 1.0 after second Pop", surface.CurrentOpacity())
	}
}

func TestPushOpacityValidation(t *testing.T) {
	surface := NewSurface(&Page{})

	tests := []struct {
		name    string
		opacity float64
		wantErr bool
	}{
		{"Valid 0.0", 0.0, false},
		{"Valid 0.5", 0.5, false},
		{"Valid 1.0", 1.0, false},
		{"Invalid negative", -0.1, true},
		{"Invalid > 1", 1.1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := surface.PushOpacity(tt.opacity)
			if (err != nil) != tt.wantErr {
				t.Errorf("PushOpacity(%f) error = %v, wantErr %v", tt.opacity, err, tt.wantErr)
			}
			// Clean up if successful
			if err == nil {
				surface.Pop()
			}
		})
	}
}

func TestPushPopBlendMode(t *testing.T) {
	surface := NewSurface(&Page{})

	// Initial blend mode should be Normal
	if surface.CurrentBlendMode() != BlendModeNormal {
		t.Errorf("Initial blend mode = %v, want Normal", surface.CurrentBlendMode())
	}

	// Push blend mode
	surface.PushBlendMode(BlendModeMultiply)

	if surface.CurrentBlendMode() != BlendModeMultiply {
		t.Errorf("CurrentBlendMode = %v, want Multiply", surface.CurrentBlendMode())
	}

	// Pop
	surface.Pop()

	if surface.CurrentBlendMode() != BlendModeNormal {
		t.Errorf("CurrentBlendMode = %v, want Normal after Pop", surface.CurrentBlendMode())
	}
}

func TestNestedPushPop(t *testing.T) {
	surface := NewSurface(&Page{})

	// Push multiple states
	surface.PushTransform(Translate(100, 0))
	if err := surface.PushOpacity(0.5); err != nil {
		t.Fatalf("PushOpacity failed: %v", err)
	}
	surface.PushBlendMode(BlendModeMultiply)

	if surface.StackDepth() != 3 {
		t.Errorf("StackDepth = %d, want 3", surface.StackDepth())
	}

	// Pop all
	surface.Pop()
	if surface.StackDepth() != 2 {
		t.Errorf("StackDepth = %d, want 2", surface.StackDepth())
	}

	surface.Pop()
	if surface.StackDepth() != 1 {
		t.Errorf("StackDepth = %d, want 1", surface.StackDepth())
	}

	surface.Pop()
	if surface.StackDepth() != 0 {
		t.Errorf("StackDepth = %d, want 0", surface.StackDepth())
	}

	// Verify all states restored
	if surface.CurrentOpacity() != 1.0 {
		t.Error("Opacity not restored to 1.0")
	}
	if surface.CurrentBlendMode() != BlendModeNormal {
		t.Error("BlendMode not restored to Normal")
	}
	if surface.CurrentTransform() != Identity() {
		t.Error("Transform not restored to Identity")
	}
}

func TestPopWithoutPushPanics(t *testing.T) {
	surface := NewSurface(&Page{})

	defer func() {
		if r := recover(); r == nil {
			t.Error("Pop() without Push did not panic")
		}
	}()

	surface.Pop() // Should panic
}

func TestPushClipPath(t *testing.T) {
	surface := NewSurface(&Page{})

	path := NewPath()

	err := surface.PushClipPath(path, FillRuleNonZero)
	if err != nil {
		t.Fatalf("PushClipPath failed: %v", err)
	}

	if surface.CurrentClipPath() != path {
		t.Error("CurrentClipPath not set correctly")
	}

	surface.Pop()

	if surface.CurrentClipPath() != nil {
		t.Error("ClipPath not cleared after Pop")
	}
}

func TestPushClipPathNil(t *testing.T) {
	surface := NewSurface(&Page{})

	err := surface.PushClipPath(nil, FillRuleNonZero)
	if err == nil {
		t.Error("PushClipPath(nil) should return error")
	}
}

func TestTransformComposition(t *testing.T) {
	surface := NewSurface(&Page{})

	// Push translate
	surface.PushTransform(Translate(100, 0))

	// Push rotate (should compose with translate)
	surface.PushTransform(Rotate(45))

	// The transforms should be composed
	transform := surface.CurrentTransform()

	// Verify it's not identity
	if transform == Identity() {
		t.Error("Composed transform should not be identity")
	}

	// Pop rotate
	surface.Pop()

	// Should have only translate now
	transform = surface.CurrentTransform()
	expected := Translate(100, 0)

	if transform != expected {
		t.Error("Transform not correctly restored after Pop")
	}

	// Pop translate
	surface.Pop()

	// Should be identity
	if surface.CurrentTransform() != Identity() {
		t.Error("Transform not restored to identity")
	}
}

func TestBlendModeString(t *testing.T) {
	tests := []struct {
		mode BlendMode
		want string
	}{
		{BlendModeNormal, "Normal"},
		{BlendModeMultiply, "Multiply"},
		{BlendModeScreen, "Screen"},
		{BlendModeOverlay, "Overlay"},
		{BlendModeDarken, "Darken"},
		{BlendModeLighten, "Lighten"},
		{BlendModeColorDodge, "ColorDodge"},
		{BlendModeColorBurn, "ColorBurn"},
		{BlendModeHardLight, "HardLight"},
		{BlendModeSoftLight, "SoftLight"},
		{BlendModeDifference, "Difference"},
		{BlendModeExclusion, "Exclusion"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.mode.String()
			if got != tt.want {
				t.Errorf("BlendMode.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGraphicsStateClone(t *testing.T) {
	original := NewGraphicsState()
	original.Opacity = 0.7
	original.BlendMode = BlendModeMultiply

	clone := original.Clone()

	// Verify values are copied
	if clone.Opacity != original.Opacity {
		t.Error("Opacity not cloned correctly")
	}
	if clone.BlendMode != original.BlendMode {
		t.Error("BlendMode not cloned correctly")
	}

	// Modify clone and verify original unchanged
	clone.Opacity = 0.3
	if original.Opacity != 0.7 {
		t.Error("Modifying clone affected original")
	}
}

func TestSurfaceSetFill(t *testing.T) {
	surface := NewSurface(&Page{})

	// Initially no fill
	if surface.CurrentFill() != nil {
		t.Error("Initial fill should be nil")
	}

	// Set fill
	fill := NewFill(Red).WithOpacity(0.8)
	surface.SetFill(fill)

	if surface.CurrentFill() != fill {
		t.Error("Fill not set correctly")
	}

	// Clear fill
	surface.SetFill(nil)
	if surface.CurrentFill() != nil {
		t.Error("Fill not cleared")
	}
}

func TestSurfaceSetStroke(t *testing.T) {
	surface := NewSurface(&Page{})

	// Initially no stroke
	if surface.CurrentStroke() != nil {
		t.Error("Initial stroke should be nil")
	}

	// Set stroke
	stroke := NewStroke(Black).WithWidth(2.0)
	surface.SetStroke(stroke)

	if surface.CurrentStroke() != stroke {
		t.Error("Stroke not set correctly")
	}

	// Clear stroke
	surface.SetStroke(nil)
	if surface.CurrentStroke() != nil {
		t.Error("Stroke not cleared")
	}
}

func TestSurfaceDrawPath(t *testing.T) {
	surface := NewSurface(&Page{})
	path := NewPath()

	// Test with nil path
	err := surface.DrawPath(nil)
	if err == nil {
		t.Error("DrawPath(nil) should return error")
	}

	// Test with valid fill
	fill := NewFill(Red).WithOpacity(0.8)
	surface.SetFill(fill)
	err = surface.DrawPath(path)
	if err != nil {
		t.Errorf("DrawPath with valid fill failed: %v", err)
	}

	// Test with valid stroke
	surface.SetFill(nil)
	stroke := NewStroke(Black).WithWidth(2.0)
	surface.SetStroke(stroke)
	err = surface.DrawPath(path)
	if err != nil {
		t.Errorf("DrawPath with valid stroke failed: %v", err)
	}

	// Test with both fill and stroke
	surface.SetFill(fill)
	err = surface.DrawPath(path)
	if err != nil {
		t.Errorf("DrawPath with both fill and stroke failed: %v", err)
	}

	// Test with invalid fill
	invalidFill := &Fill{
		Paint:   Red,
		Opacity: 1.5, // Invalid
		Rule:    FillRuleNonZero,
	}
	surface.SetFill(invalidFill)
	err = surface.DrawPath(path)
	if err == nil {
		t.Error("DrawPath with invalid fill should return error")
	}

	// Test with invalid stroke
	surface.SetFill(nil)
	invalidStroke := &Stroke{
		Paint: Black,
		Width: -1.0, // Invalid
	}
	surface.SetStroke(invalidStroke)
	err = surface.DrawPath(path)
	if err == nil {
		t.Error("DrawPath with invalid stroke should return error")
	}
}

func TestSurfaceDrawRect(t *testing.T) {
	surface := NewSurface(&Page{})

	tests := []struct {
		name    string
		rect    Rect
		wantErr bool
	}{
		{
			name:    "Valid rect",
			rect:    Rect{X: 50, Y: 50, Width: 100, Height: 100},
			wantErr: false,
		},
		{
			name:    "Invalid width <= 0",
			rect:    Rect{X: 50, Y: 50, Width: 0, Height: 100},
			wantErr: true,
		},
		{
			name:    "Invalid negative width",
			rect:    Rect{X: 50, Y: 50, Width: -10, Height: 100},
			wantErr: true,
		},
		{
			name:    "Invalid height <= 0",
			rect:    Rect{X: 50, Y: 50, Width: 100, Height: 0},
			wantErr: true,
		},
		{
			name:    "Invalid negative height",
			rect:    Rect{X: 50, Y: 50, Width: 100, Height: -10},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set valid fill and stroke
			fill := NewFill(Red)
			stroke := NewStroke(Black)
			surface.SetFill(fill)
			surface.SetStroke(stroke)

			err := surface.DrawRect(tt.rect)
			if (err != nil) != tt.wantErr {
				t.Errorf("DrawRect() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	// Test with invalid fill
	invalidFill := &Fill{
		Paint:   Red,
		Opacity: 1.5, // Invalid
		Rule:    FillRuleNonZero,
	}
	surface.SetFill(invalidFill)
	err := surface.DrawRect(Rect{X: 0, Y: 0, Width: 100, Height: 100})
	if err == nil {
		t.Error("DrawRect with invalid fill should return error")
	}

	// Test with invalid stroke
	surface.SetFill(nil)
	invalidStroke := &Stroke{
		Paint: Black,
		Width: -1.0, // Invalid
	}
	surface.SetStroke(invalidStroke)
	err = surface.DrawRect(Rect{X: 0, Y: 0, Width: 100, Height: 100})
	if err == nil {
		t.Error("DrawRect with invalid stroke should return error")
	}
}

func TestSurfaceFillStrokePersistence(t *testing.T) {
	surface := NewSurface(&Page{})

	fill := NewFill(Red)
	stroke := NewStroke(Black)

	surface.SetFill(fill)
	surface.SetStroke(stroke)

	// Push state should preserve fill/stroke
	surface.PushOpacity(0.5)

	if surface.CurrentFill() != fill {
		t.Error("Fill not preserved after PushOpacity")
	}
	if surface.CurrentStroke() != stroke {
		t.Error("Stroke not preserved after PushOpacity")
	}

	// Modify in nested state
	newFill := NewFill(Blue)
	surface.SetFill(newFill)

	if surface.CurrentFill() != newFill {
		t.Error("Fill not updated in nested state")
	}

	// Pop should restore original
	surface.Pop()

	if surface.CurrentFill() != fill {
		t.Error("Fill not restored after Pop")
	}
}
