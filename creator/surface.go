package creator

import (
	"errors"
	"fmt"
)

// Surface represents a drawing surface with a graphics state stack.
//
// Surface provides Skia-like Push/Pop semantics for graphics state management.
// This allows composable transformations, opacity, blend modes, and clipping.
//
// Example:
//
//	surface := page.Surface()
//
//	// Draw with transform and opacity
//	surface.PushTransform(Rotate(45))
//	surface.PushOpacity(0.5)
//	surface.DrawRect(rect)
//	surface.Pop()  // Restore opacity
//	surface.Pop()  // Restore transform
type Surface struct {
	// page is the underlying page this surface draws on.
	page *Page

	// stateStack is the graphics state stack.
	// Each Push* operation saves the current state and pushes a new one.
	// Pop restores the previous state.
	stateStack []GraphicsState

	// currentState is the active graphics state.
	currentState GraphicsState
}

// NewSurface creates a new drawing surface for a page.
//
// The surface starts with an empty state stack and default graphics state.
func NewSurface(page *Page) *Surface {
	return &Surface{
		page:         page,
		stateStack:   make([]GraphicsState, 0, 8), // Pre-allocate for common depth
		currentState: NewGraphicsState(),
	}
}

// PushTransform saves the current state and applies a transformation.
//
// The transformation is applied to all subsequent drawing operations.
// Call Pop() to restore the previous state.
//
// Example:
//
//	surface.PushTransform(Rotate(45))
//	surface.DrawRect(rect)  // Drawn rotated
//	surface.Pop()           // Restore rotation
func (s *Surface) PushTransform(t Transform) {
	// Save current state
	s.stateStack = append(s.stateStack, s.currentState.Clone())

	// Apply transformation (compose with current transform)
	s.currentState.Transform = s.currentState.Transform.Then(t)
}

// PushOpacity saves the current state and applies an opacity.
//
// The opacity is multiplicative with the current opacity.
// Call Pop() to restore the previous opacity.
//
// Example:
//
//	surface.PushOpacity(0.5)
//	surface.PushOpacity(0.5)  // Combined opacity = 0.25
//	surface.DrawRect(rect)
//	surface.Pop()  // opacity = 0.5
//	surface.Pop()  // opacity = 1.0
func (s *Surface) PushOpacity(opacity float64) error {
	if opacity < 0 || opacity > 1 {
		return errors.New("opacity must be in range [0.0, 1.0]")
	}

	// Save current state
	s.stateStack = append(s.stateStack, s.currentState.Clone())

	// Apply opacity (multiplicative)
	s.currentState.Opacity *= opacity

	return nil
}

// PushBlendMode saves the current state and applies a blend mode.
//
// The blend mode controls how colors blend with the background.
// Call Pop() to restore the previous blend mode.
//
// Example:
//
//	surface.PushBlendMode(BlendModeMultiply)
//	surface.DrawRect(rect)
//	surface.Pop()
func (s *Surface) PushBlendMode(mode BlendMode) {
	// Save current state
	s.stateStack = append(s.stateStack, s.currentState.Clone())

	// Set blend mode
	s.currentState.BlendMode = mode
}

// PushClipPath saves the current state and applies a clipping path.
//
// All subsequent drawing is clipped to the path.
// Call Pop() to restore the previous clipping state.
//
// This method will be implemented in feat-053 (ClipPath).
//
// Example:
//
//	path := NewPath()
//	path.MoveTo(0, 0)
//	path.LineTo(100, 0)
//	path.LineTo(100, 100)
//	path.Close()
//
//	surface.PushClipPath(path, FillRuleNonZero)
//	surface.DrawRect(rect)  // Clipped to triangle
//	surface.Pop()
func (s *Surface) PushClipPath(path *Path, rule FillRule) error {
	if path == nil {
		return errors.New("clip path cannot be nil")
	}

	// Save current state
	s.stateStack = append(s.stateStack, s.currentState.Clone())

	// Set clip path
	s.currentState.ClipPath = path

	// Note: FillRule will be stored in Path when feat-053 is implemented
	_ = rule

	return nil
}

// Pop restores the previous graphics state.
//
// Each Pop must match a previous Push* call.
// Panics if Pop is called more times than Push.
//
// Example:
//
//	surface.PushOpacity(0.5)
//	surface.DrawRect(rect)
//	surface.Pop()  // OK
//	surface.Pop()  // PANIC: no matching Push
func (s *Surface) Pop() {
	if len(s.stateStack) == 0 {
		panic("Pop() called without matching Push*()")
	}

	// Restore previous state
	s.currentState = s.stateStack[len(s.stateStack)-1]
	s.stateStack = s.stateStack[:len(s.stateStack)-1]
}

// CurrentTransform returns the current transformation matrix.
func (s *Surface) CurrentTransform() Transform {
	return s.currentState.Transform
}

// CurrentOpacity returns the current opacity value.
func (s *Surface) CurrentOpacity() float64 {
	return s.currentState.Opacity
}

// CurrentBlendMode returns the current blend mode.
func (s *Surface) CurrentBlendMode() BlendMode {
	return s.currentState.BlendMode
}

// CurrentClipPath returns the current clipping path (nil if no clipping).
func (s *Surface) CurrentClipPath() *Path {
	return s.currentState.ClipPath
}

// StackDepth returns the number of saved states on the stack.
//
// This is useful for debugging and ensuring Push/Pop are balanced.
func (s *Surface) StackDepth() int {
	return len(s.stateStack)
}

// SetFill sets the current fill configuration.
//
// All subsequent drawing operations will use this fill.
// Pass nil to disable filling.
//
// Example:
//
//	fill := NewFill(Red).WithOpacity(0.8)
//	surface.SetFill(fill)
//	surface.DrawRect(rect)  // Filled with red at 80% opacity
func (s *Surface) SetFill(fill *Fill) {
	s.currentState.Fill = fill
}

// SetStroke sets the current stroke configuration.
//
// All subsequent drawing operations will use this stroke.
// Pass nil to disable stroking.
//
// Example:
//
//	stroke := NewStroke(Black).WithWidth(2.0)
//	surface.SetStroke(stroke)
//	surface.DrawRect(rect)  // Stroked with black 2pt line
func (s *Surface) SetStroke(stroke *Stroke) {
	s.currentState.Stroke = stroke
}

// DrawPath draws a path with the current fill and stroke.
//
// The path is drawn using the current graphics state.
// If both fill and stroke are set, the path is filled then stroked (B operator).
// If only fill is set, the path is filled (f operator).
// If only stroke is set, the path is stroked (S operator).
// If neither is set, no drawing occurs (but path is constructed).
//
// Parameters:
//   - path: Path to draw
//
// Example:
//
//	path := NewPath()
//	path.MoveTo(0, 0)
//	path.LineTo(100, 0)
//	path.LineTo(50, 100)
//	path.Close()
//
//	surface.SetFill(NewFill(Red))
//	surface.SetStroke(NewStroke(Black).WithWidth(2))
//	surface.DrawPath(path)
func (s *Surface) DrawPath(path *Path) error {
	if path == nil {
		return errors.New("path cannot be nil")
	}

	if path.IsEmpty() {
		// Empty path, nothing to draw
		return nil
	}

	hasFill := s.currentState.Fill != nil
	hasStroke := s.currentState.Stroke != nil

	if !hasFill && !hasStroke {
		// Nothing to do (no fill, no stroke)
		return nil
	}

	// Validate fill and stroke configurations
	if hasFill {
		if err := s.currentState.Fill.Validate(); err != nil {
			return fmt.Errorf("invalid fill: %w", err)
		}
	}

	if hasStroke {
		if err := s.currentState.Stroke.Validate(); err != nil {
			return fmt.Errorf("invalid stroke: %w", err)
		}
	}

	// TODO: Implement actual PDF content stream generation
	// For now, this validates and prepares for rendering
	// Full implementation will add to page.content stream:
	//   1. Set fill color/pattern
	//   2. Set stroke color/pattern + line width/cap/join/dash
	//   3. path.toPDFOperators()
	//   4. Painting operator (f, S, B, f*, S*, B*)

	return nil
}

// FillPath fills a path using the current fill configuration.
//
// The path is filled using the current fill color/gradient and fill rule.
// Stroke configuration is ignored (even if set).
//
// PDF operators: f (NonZero) or f* (EvenOdd)
//
// Parameters:
//   - path: Path to fill
//
// Example:
//
//	path := NewPath().
//	    AddCircle(Point{150, 150}, 50)
//
//	surface.SetFill(NewFill(Red))
//	surface.FillPath(path)
func (s *Surface) FillPath(path *Path) error {
	if path == nil {
		return errors.New("path cannot be nil")
	}

	if path.IsEmpty() {
		return nil
	}

	if s.currentState.Fill == nil {
		return errors.New("no fill configuration set (call SetFill first)")
	}

	if err := s.currentState.Fill.Validate(); err != nil {
		return fmt.Errorf("invalid fill: %w", err)
	}

	// TODO: Implement actual PDF content stream generation
	// For now, this validates and prepares for rendering
	// Full implementation will add to page.content stream:
	//   1. Set fill color/pattern
	//   2. path.toPDFOperators()
	//   3. Fill operator (f or f* based on fill rule)

	return nil
}

// StrokePath strokes a path using the current stroke configuration.
//
// The path is stroked using the current stroke color/gradient, line width,
// cap style, join style, and dash pattern.
// Fill configuration is ignored (even if set).
//
// PDF operator: S
//
// Parameters:
//   - path: Path to stroke
//
// Example:
//
//	path := NewPath().
//	    MoveTo(50, 50).
//	    LineTo(150, 50).
//	    LineTo(100, 150).
//	    Close()
//
//	stroke := NewStroke(Black).
//	    WithWidth(3.0).
//	    WithLineCap(LineCapRound).
//	    WithLineJoin(LineJoinRound)
//	surface.SetStroke(stroke)
//	surface.StrokePath(path)
func (s *Surface) StrokePath(path *Path) error {
	if path == nil {
		return errors.New("path cannot be nil")
	}

	if path.IsEmpty() {
		return nil
	}

	if s.currentState.Stroke == nil {
		return errors.New("no stroke configuration set (call SetStroke first)")
	}

	if err := s.currentState.Stroke.Validate(); err != nil {
		return fmt.Errorf("invalid stroke: %w", err)
	}

	// TODO: Implement actual PDF content stream generation
	// For now, this validates and prepares for rendering
	// Full implementation will add to page.content stream:
	//   1. Set stroke color/pattern
	//   2. Set line width, cap, join, miter limit, dash
	//   3. path.toPDFOperators()
	//   4. Stroke operator (S)

	return nil
}

// Rect represents a rectangle in user space.
type Rect struct {
	X      float64 // Left edge
	Y      float64 // Bottom edge
	Width  float64 // Width
	Height float64 // Height
}

// DrawRect draws a rectangle with the current fill and stroke.
//
// The rectangle is drawn using the current graphics state.
// If both fill and stroke are set, the rectangle is filled then stroked.
//
// Parameters:
//   - rect: Rectangle to draw
//
// Example:
//
//	surface.SetFill(NewFill(Red))
//	surface.SetStroke(NewStroke(Black).WithWidth(2))
//	surface.DrawRect(Rect{X: 50, Y: 50, Width: 100, Height: 100})
func (s *Surface) DrawRect(rect Rect) error {
	if rect.Width <= 0 {
		return fmt.Errorf("rect width must be positive, got: %f", rect.Width)
	}
	if rect.Height <= 0 {
		return fmt.Errorf("rect height must be positive, got: %f", rect.Height)
	}

	// TODO: Implement actual PDF drawing in a later phase
	// For now, this validates the current state

	if s.currentState.Fill != nil {
		if err := s.currentState.Fill.Validate(); err != nil {
			return fmt.Errorf("invalid fill: %w", err)
		}
	}

	if s.currentState.Stroke != nil {
		if err := s.currentState.Stroke.Validate(); err != nil {
			return fmt.Errorf("invalid stroke: %w", err)
		}
	}

	return nil
}

// CurrentFill returns the current fill configuration.
func (s *Surface) CurrentFill() *Fill {
	return s.currentState.Fill
}

// CurrentStroke returns the current stroke configuration.
func (s *Surface) CurrentStroke() *Stroke {
	return s.currentState.Stroke
}

// Errors
var (
	// ErrPopWithoutPush is returned when Pop is called without a matching Push.
	ErrPopWithoutPush = errors.New("Pop() called without matching Push*()")

	// ErrInvalidRect is returned when a rectangle has invalid dimensions.
	ErrInvalidRect = errors.New("invalid rectangle dimensions")
)
