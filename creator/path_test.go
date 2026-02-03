package creator

import (
	"math"
	"strings"
	"testing"
)

// Test NewPath creates an empty path
func TestNewPath(t *testing.T) {
	path := NewPath()
	if path == nil {
		t.Fatal("NewPath returned nil")
	}
	if !path.IsEmpty() {
		t.Error("NewPath should create empty path")
	}
	if path.hasCurrentPoint {
		t.Error("NewPath should have no current point")
	}
}

// Test MoveTo
func TestPath_MoveTo(t *testing.T) {
	path := NewPath().MoveTo(100, 200)

	if path.IsEmpty() {
		t.Error("Path should not be empty after MoveTo")
	}
	if !path.hasCurrentPoint {
		t.Error("MoveTo should set current point")
	}
	if path.currentPoint.X != 100 || path.currentPoint.Y != 200 {
		t.Errorf("MoveTo: expected (100, 200), got (%f, %f)",
			path.currentPoint.X, path.currentPoint.Y)
	}
	if path.subpathStart.X != 100 || path.subpathStart.Y != 200 {
		t.Error("MoveTo should set subpath start")
	}
}

// Test LineTo
func TestPath_LineTo(t *testing.T) {
	path := NewPath().
		MoveTo(100, 100).
		LineTo(200, 100).
		LineTo(200, 200)

	if len(path.commands) != 3 {
		t.Errorf("Expected 3 commands, got %d", len(path.commands))
	}
	if path.currentPoint.X != 200 || path.currentPoint.Y != 200 {
		t.Errorf("LineTo: expected (200, 200), got (%f, %f)",
			path.currentPoint.X, path.currentPoint.Y)
	}
}

// Test LineTo panics without current point
func TestPath_LineTo_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("LineTo should panic without current point")
		}
	}()

	path := NewPath()
	path.LineTo(100, 100) // Should panic
}

// Test CubicTo
func TestPath_CubicTo(t *testing.T) {
	path := NewPath().
		MoveTo(100, 100).
		CubicTo(150, 50, 200, 150, 250, 100)

	if len(path.commands) != 2 {
		t.Errorf("Expected 2 commands, got %d", len(path.commands))
	}
	if path.commands[1].op != pathOpCubicTo {
		t.Error("Second command should be CubicTo")
	}
	if len(path.commands[1].args) != 6 {
		t.Errorf("CubicTo should have 6 args, got %d", len(path.commands[1].args))
	}
	if path.currentPoint.X != 250 || path.currentPoint.Y != 100 {
		t.Errorf("CubicTo: expected (250, 100), got (%f, %f)",
			path.currentPoint.X, path.currentPoint.Y)
	}
}

// Test CubicTo panics without current point
func TestPath_CubicTo_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("CubicTo should panic without current point")
		}
	}()

	path := NewPath()
	path.CubicTo(150, 50, 200, 150, 250, 100) // Should panic
}

// Test QuadraticTo converts to cubic correctly
func TestPath_QuadraticTo(t *testing.T) {
	path := NewPath().
		MoveTo(100, 100).
		QuadraticTo(150, 50, 200, 100)

	if len(path.commands) != 2 {
		t.Errorf("Expected 2 commands, got %d", len(path.commands))
	}

	// QuadraticTo should convert to CubicTo internally
	if path.commands[1].op != pathOpCubicTo {
		t.Error("QuadraticTo should convert to CubicTo")
	}
	if len(path.commands[1].args) != 6 {
		t.Errorf("CubicTo should have 6 args, got %d", len(path.commands[1].args))
	}

	// Verify conversion formula: CP1 = P0 + 2/3 * (CP - P0)
	// P0 = (100, 100), CP = (150, 50), P1 = (200, 100)
	expectedCP1x := 100 + (2.0/3.0)*(150-100) // 133.33
	expectedCP1y := 100 + (2.0/3.0)*(50-100)  // 66.67
	expectedCP2x := 200 + (2.0/3.0)*(150-200) // 166.67
	expectedCP2y := 100 + (2.0/3.0)*(50-100)  // 66.67

	cp1x := path.commands[1].args[0]
	cp1y := path.commands[1].args[1]
	cp2x := path.commands[1].args[2]
	cp2y := path.commands[1].args[3]

	if math.Abs(cp1x-expectedCP1x) > 0.01 || math.Abs(cp1y-expectedCP1y) > 0.01 {
		t.Errorf("QuadraticTo CP1: expected (%.2f, %.2f), got (%.2f, %.2f)",
			expectedCP1x, expectedCP1y, cp1x, cp1y)
	}
	if math.Abs(cp2x-expectedCP2x) > 0.01 || math.Abs(cp2y-expectedCP2y) > 0.01 {
		t.Errorf("QuadraticTo CP2: expected (%.2f, %.2f), got (%.2f, %.2f)",
			expectedCP2x, expectedCP2y, cp2x, cp2y)
	}
}

// Test QuadraticTo panics without current point
func TestPath_QuadraticTo_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("QuadraticTo should panic without current point")
		}
	}()

	path := NewPath()
	path.QuadraticTo(150, 50, 200, 100) // Should panic
}

// Test Close
func TestPath_Close(t *testing.T) {
	path := NewPath().
		MoveTo(100, 100).
		LineTo(200, 100).
		LineTo(150, 200).
		Close()

	if len(path.commands) != 4 {
		t.Errorf("Expected 4 commands, got %d", len(path.commands))
	}
	if path.commands[3].op != pathOpClose {
		t.Error("Last command should be Close")
	}
	if path.hasCurrentPoint {
		t.Error("Close should invalidate current point")
	}
}

// Test Close with no current point is safe
func TestPath_Close_NoCurrentPoint(t *testing.T) {
	path := NewPath().Close() // Should not panic

	if !path.IsEmpty() {
		t.Error("Close on empty path should remain empty")
	}
}

// Test AddRect
func TestPath_AddRect(t *testing.T) {
	rect := Rect{X: 10, Y: 20, Width: 100, Height: 50}
	path := NewPath().AddRect(rect)

	if len(path.commands) != 1 {
		t.Errorf("Expected 1 command, got %d", len(path.commands))
	}
	if path.commands[0].op != pathOpRect {
		t.Error("Command should be Rect")
	}
	if len(path.commands[0].args) != 4 {
		t.Errorf("Rect should have 4 args, got %d", len(path.commands[0].args))
	}

	args := path.commands[0].args
	if args[0] != rect.X || args[1] != rect.Y || args[2] != rect.Width || args[3] != rect.Height {
		t.Error("Rect args do not match")
	}

	// Rectangle is closed, so no current point
	if path.hasCurrentPoint {
		t.Error("AddRect should not have current point")
	}
}

// Test AddRoundedRect with zero radius becomes regular rect
func TestPath_AddRoundedRect_ZeroRadius(t *testing.T) {
	rect := Rect{X: 10, Y: 10, Width: 100, Height: 50}
	path := NewPath().AddRoundedRect(rect, 0)

	// Should use AddRect optimization
	if len(path.commands) != 1 || path.commands[0].op != pathOpRect {
		t.Error("AddRoundedRect with zero radius should use AddRect")
	}
}

// Test AddRoundedRect with valid radius
func TestPath_AddRoundedRect(t *testing.T) {
	rect := Rect{X: 10, Y: 10, Width: 100, Height: 50}
	path := NewPath().AddRoundedRect(rect, 10)

	if len(path.commands) < 5 {
		t.Errorf("AddRoundedRect should have multiple commands, got %d", len(path.commands))
	}

	// Should start with MoveTo
	if path.commands[0].op != pathOpMoveTo {
		t.Error("AddRoundedRect should start with MoveTo")
	}

	// Should end with Close
	if path.commands[len(path.commands)-1].op != pathOpClose {
		t.Error("AddRoundedRect should end with Close")
	}
}

// Test AddCircle
func TestPath_AddCircle(t *testing.T) {
	center := Point{X: 150, Y: 150}
	radius := 50.0
	path := NewPath().AddCircle(center, radius)

	if len(path.commands) < 5 {
		t.Errorf("AddCircle should have multiple commands, got %d", len(path.commands))
	}

	// Should start with MoveTo
	if path.commands[0].op != pathOpMoveTo {
		t.Error("AddCircle should start with MoveTo")
	}

	// Should have multiple CubicTo commands (4 for a circle)
	cubicCount := 0
	for _, cmd := range path.commands {
		if cmd.op == pathOpCubicTo {
			cubicCount++
		}
	}
	if cubicCount != 4 {
		t.Errorf("AddCircle should have 4 CubicTo commands, got %d", cubicCount)
	}

	// Should end with Close
	if path.commands[len(path.commands)-1].op != pathOpClose {
		t.Error("AddCircle should end with Close")
	}
}

// Test AddCircle panics with non-positive radius
func TestPath_AddCircle_InvalidRadius(t *testing.T) {
	tests := []struct {
		name   string
		radius float64
	}{
		{"zero radius", 0},
		{"negative radius", -50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("AddCircle should panic with radius %f", tt.radius)
				}
			}()

			path := NewPath()
			path.AddCircle(Point{150, 150}, tt.radius) // Should panic
		})
	}
}

// Test AddEllipse
func TestPath_AddEllipse(t *testing.T) {
	rect := Rect{X: 10, Y: 10, Width: 200, Height: 100}
	path := NewPath().AddEllipse(rect)

	if len(path.commands) < 5 {
		t.Errorf("AddEllipse should have multiple commands, got %d", len(path.commands))
	}

	// Should start with MoveTo (at rightmost point)
	if path.commands[0].op != pathOpMoveTo {
		t.Error("AddEllipse should start with MoveTo")
	}

	// Check first point is at rightmost position (3 o'clock)
	cx := rect.X + rect.Width/2  // 110
	cy := rect.Y + rect.Height/2 // 60
	rx := rect.Width / 2         // 100
	expectedX := cx + rx         // 210

	firstX := path.commands[0].args[0]
	firstY := path.commands[0].args[1]

	if math.Abs(firstX-expectedX) > 0.01 || math.Abs(firstY-cy) > 0.01 {
		t.Errorf("AddEllipse first point: expected (%.2f, %.2f), got (%.2f, %.2f)",
			expectedX, cy, firstX, firstY)
	}

	// Should end with Close
	if path.commands[len(path.commands)-1].op != pathOpClose {
		t.Error("AddEllipse should end with Close")
	}
}

// Test AddArc
func TestPath_AddArc(t *testing.T) {
	center := Point{X: 150, Y: 150}
	radius := 50.0
	path := NewPath().AddArc(center, radius, 0, 90) // Quarter circle

	if len(path.commands) < 2 {
		t.Errorf("AddArc should have at least 2 commands, got %d", len(path.commands))
	}

	// Should start with MoveTo (no current point initially)
	if path.commands[0].op != pathOpMoveTo {
		t.Error("AddArc should start with MoveTo when no current point")
	}

	// Should have CubicTo commands for the arc
	hasCubic := false
	for _, cmd := range path.commands {
		if cmd.op == pathOpCubicTo {
			hasCubic = true
			break
		}
	}
	if !hasCubic {
		t.Error("AddArc should have CubicTo commands")
	}
}

// Test AddArc with current point draws line first
func TestPath_AddArc_WithCurrentPoint(t *testing.T) {
	center := Point{X: 150, Y: 150}
	radius := 50.0
	path := NewPath().
		MoveTo(50, 50).
		AddArc(center, radius, 0, 90)

	// Should have MoveTo, then LineTo (to arc start), then CubicTo
	if len(path.commands) < 3 {
		t.Errorf("AddArc with current point should have at least 3 commands, got %d", len(path.commands))
	}

	// Second command should be LineTo (connecting current point to arc start)
	if path.commands[1].op != pathOpLineTo {
		t.Error("AddArc with current point should have LineTo to arc start")
	}
}

// Test AddArc panics with invalid radius
func TestPath_AddArc_InvalidRadius(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("AddArc should panic with non-positive radius")
		}
	}()

	path := NewPath()
	path.AddArc(Point{150, 150}, 0, 0, 90) // Should panic
}

// Test IsEmpty
func TestPath_IsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		build    func() *Path
		expected bool
	}{
		{
			name:     "new path",
			build:    func() *Path { return NewPath() },
			expected: true,
		},
		{
			name:     "path with MoveTo",
			build:    func() *Path { return NewPath().MoveTo(0, 0) },
			expected: false,
		},
		{
			name:     "path with LineTo",
			build:    func() *Path { return NewPath().MoveTo(0, 0).LineTo(100, 100) },
			expected: false,
		},
		{
			name:     "path with AddRect",
			build:    func() *Path { return NewPath().AddRect(Rect{X: 0, Y: 0, Width: 100, Height: 100}) },
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.build()
			if path.IsEmpty() != tt.expected {
				t.Errorf("IsEmpty: expected %v, got %v", tt.expected, path.IsEmpty())
			}
		})
	}
}

// Test Bounds
func TestPath_Bounds(t *testing.T) {
	tests := []struct {
		name     string
		build    func() *Path
		expected Rect
	}{
		{
			name:     "empty path",
			build:    func() *Path { return NewPath() },
			expected: Rect{},
		},
		{
			name: "simple line",
			build: func() *Path {
				return NewPath().MoveTo(10, 20).LineTo(100, 80)
			},
			expected: Rect{X: 10, Y: 20, Width: 90, Height: 60},
		},
		{
			name: "triangle",
			build: func() *Path {
				return NewPath().
					MoveTo(50, 10).
					LineTo(90, 90).
					LineTo(10, 90).
					Close()
			},
			expected: Rect{X: 10, Y: 10, Width: 80, Height: 80},
		},
		{
			name: "rectangle",
			build: func() *Path {
				return NewPath().AddRect(Rect{X: 20, Y: 30, Width: 100, Height: 50})
			},
			expected: Rect{X: 20, Y: 30, Width: 100, Height: 50},
		},
		{
			name: "cubic bezier",
			build: func() *Path {
				return NewPath().
					MoveTo(0, 0).
					CubicTo(50, -50, 100, 150, 150, 100)
			},
			// Bounds include control points
			expected: Rect{X: 0, Y: -50, Width: 150, Height: 200},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.build()
			bounds := path.Bounds()

			if math.Abs(bounds.X-tt.expected.X) > 0.01 ||
				math.Abs(bounds.Y-tt.expected.Y) > 0.01 ||
				math.Abs(bounds.Width-tt.expected.Width) > 0.01 ||
				math.Abs(bounds.Height-tt.expected.Height) > 0.01 {
				t.Errorf("Bounds: expected %+v, got %+v", tt.expected, bounds)
			}
		})
	}
}

// Test Clone
func TestPath_Clone(t *testing.T) {
	original := NewPath().
		MoveTo(100, 100).
		LineTo(200, 100).
		LineTo(200, 200).
		Close()

	clone := original.Clone()

	// Clone should be independent
	if clone == original {
		t.Error("Clone should create a new path instance")
	}

	// Clone should have same commands
	if len(clone.commands) != len(original.commands) {
		t.Errorf("Clone: expected %d commands, got %d", len(original.commands), len(clone.commands))
	}

	for i := range original.commands {
		if clone.commands[i].op != original.commands[i].op {
			t.Errorf("Clone: command %d op mismatch", i)
		}
		if len(clone.commands[i].args) != len(original.commands[i].args) {
			t.Errorf("Clone: command %d args length mismatch", i)
		}
	}

	// Modifying clone should not affect original
	// Need to call MoveTo first since the cloned path has no current point (was closed)
	clone.MoveTo(300, 300).LineTo(400, 400)

	if len(clone.commands) == len(original.commands) {
		t.Error("Modifying clone should not affect original")
	}
}

// Test toPDFOperators
func TestPath_toPDFOperators(t *testing.T) {
	tests := []struct {
		name     string
		build    func() *Path
		expected []string // Expected operators
	}{
		{
			name:     "empty path",
			build:    func() *Path { return NewPath() },
			expected: []string{},
		},
		{
			name: "simple line",
			build: func() *Path {
				return NewPath().MoveTo(100, 100).LineTo(200, 200)
			},
			expected: []string{"100.00 100.00 m", "200.00 200.00 l"},
		},
		{
			name: "closed triangle",
			build: func() *Path {
				return NewPath().
					MoveTo(50, 50).
					LineTo(150, 50).
					LineTo(100, 150).
					Close()
			},
			expected: []string{
				"50.00 50.00 m",
				"150.00 50.00 l",
				"100.00 150.00 l",
				"h",
			},
		},
		{
			name: "rectangle",
			build: func() *Path {
				return NewPath().AddRect(Rect{X: 10, Y: 20, Width: 100, Height: 50})
			},
			expected: []string{"10.00 20.00 100.00 50.00 re"},
		},
		{
			name: "cubic bezier",
			build: func() *Path {
				return NewPath().
					MoveTo(0, 0).
					CubicTo(50, 0, 100, 100, 100, 50)
			},
			expected: []string{
				"0.00 0.00 m",
				"50.00 0.00 100.00 100.00 100.00 50.00 c",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.build()
			output := path.toPDFOperators()

			if len(tt.expected) == 0 {
				if output != "" {
					t.Errorf("Expected empty output, got: %q", output)
				}
				return
			}

			// Check that all expected operators are present
			for _, expected := range tt.expected {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q, got:\n%s", expected, output)
				}
			}
		})
	}
}

// Test fluent API chaining
func TestPath_FluentAPI(t *testing.T) {
	// Test that all methods return *Path for chaining
	path := NewPath().
		MoveTo(0, 0).
		LineTo(100, 0).
		LineTo(100, 100).
		CubicTo(150, 150, 50, 150, 0, 100).
		QuadraticTo(50, 50, 0, 0).
		Close().
		AddRect(Rect{X: 200, Y: 200, Width: 50, Height: 50}).
		AddCircle(Point{300, 300}, 25).
		AddEllipse(Rect{X: 400, Y: 400, Width: 100, Height: 50})

	if path == nil {
		t.Error("Fluent API should return path for chaining")
	}

	if len(path.commands) == 0 {
		t.Error("Fluent API chain should build path")
	}
}

// Benchmark Path creation
func BenchmarkPath_SimpleLine(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewPath().MoveTo(0, 0).LineTo(100, 100)
	}
}

func BenchmarkPath_ComplexPath(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewPath().
			MoveTo(0, 0).
			LineTo(100, 0).
			LineTo(100, 100).
			CubicTo(150, 150, 50, 150, 0, 100).
			QuadraticTo(50, 50, 0, 0).
			Close()
	}
}

func BenchmarkPath_AddCircle(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewPath().AddCircle(Point{150, 150}, 50)
	}
}

func BenchmarkPath_AddRect(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewPath().AddRect(Rect{X: 0, Y: 0, Width: 100, Height: 100})
	}
}

func BenchmarkPath_Clone(b *testing.B) {
	path := NewPath().
		MoveTo(0, 0).
		LineTo(100, 0).
		LineTo(100, 100).
		LineTo(0, 100).
		Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = path.Clone()
	}
}

func BenchmarkPath_Bounds(b *testing.B) {
	path := NewPath().
		MoveTo(0, 0).
		LineTo(100, 0).
		LineTo(100, 100).
		LineTo(0, 100).
		Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = path.Bounds()
	}
}

func BenchmarkPath_ToPDFOperators(b *testing.B) {
	path := NewPath().
		MoveTo(0, 0).
		LineTo(100, 0).
		LineTo(100, 100).
		LineTo(0, 100).
		Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = path.toPDFOperators()
	}
}
