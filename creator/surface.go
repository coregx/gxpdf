package creator

import "errors"

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

// Errors
var (
	// ErrPopWithoutPush is returned when Pop is called without a matching Push.
	ErrPopWithoutPush = errors.New("Pop() called without matching Push*()")
)
