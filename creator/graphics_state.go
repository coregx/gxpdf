package creator

// GraphicsState represents the complete graphics state at a point in time.
//
// This includes transformation, opacity, blend mode, clipping path, fill, and stroke.
// The state is saved/restored using the PDF graphics state stack (q/Q operators).
type GraphicsState struct {
	// Transform is the current transformation matrix (CTM).
	Transform Transform

	// Opacity is the overall transparency (0.0 = invisible, 1.0 = opaque).
	// This affects both fill and stroke operations.
	Opacity float64

	// BlendMode controls how colors blend with the background.
	BlendMode BlendMode

	// ClipPath is the active clipping path (nil = no clipping).
	// Drawing operations are clipped to this path.
	ClipPath *Path

	// Fill is the current fill configuration.
	Fill *Fill

	// Stroke is the current stroke configuration.
	Stroke *Stroke
}

// BlendMode defines how colors blend with the background.
type BlendMode int

const (
	// BlendModeNormal is the default blend mode (simple alpha compositing).
	BlendModeNormal BlendMode = iota

	// BlendModeMultiply multiplies the source and destination colors.
	BlendModeMultiply

	// BlendModeScreen inverts, multiplies, and inverts again.
	BlendModeScreen

	// BlendModeOverlay combines Multiply and Screen based on destination.
	BlendModeOverlay

	// BlendModeDarken selects the darker of source and destination.
	BlendModeDarken

	// BlendModeLighten selects the lighter of source and destination.
	BlendModeLighten

	// BlendModeColorDodge brightens the destination based on source.
	BlendModeColorDodge

	// BlendModeColorBurn darkens the destination based on source.
	BlendModeColorBurn

	// BlendModeHardLight is like Overlay but uses source instead of destination.
	BlendModeHardLight

	// BlendModeSoftLight is a softer version of HardLight.
	BlendModeSoftLight

	// BlendModeDifference subtracts the darker from the lighter.
	BlendModeDifference

	// BlendModeExclusion is like Difference but with lower contrast.
	BlendModeExclusion
)

// String returns the PDF blend mode name.
func (b BlendMode) String() string {
	switch b {
	case BlendModeNormal:
		return "Normal"
	case BlendModeMultiply:
		return "Multiply"
	case BlendModeScreen:
		return "Screen"
	case BlendModeOverlay:
		return "Overlay"
	case BlendModeDarken:
		return "Darken"
	case BlendModeLighten:
		return "Lighten"
	case BlendModeColorDodge:
		return "ColorDodge"
	case BlendModeColorBurn:
		return "ColorBurn"
	case BlendModeHardLight:
		return "HardLight"
	case BlendModeSoftLight:
		return "SoftLight"
	case BlendModeDifference:
		return "Difference"
	case BlendModeExclusion:
		return "Exclusion"
	default:
		return "Normal"
	}
}

// NewGraphicsState creates a new graphics state with default values.
func NewGraphicsState() GraphicsState {
	return GraphicsState{
		Transform: Identity(),
		Opacity:   1.0,
		BlendMode: BlendModeNormal,
		ClipPath:  nil,
		Fill:      nil,
		Stroke:    nil,
	}
}

// Clone creates a copy of the graphics state.
//
// This is used when pushing state onto the stack.
// Fill, Stroke, and ClipPath pointers are copied (shallow copy),
// so modifications to the pointed-to objects will affect both states.
// However, replacing the pointer (e.g., SetFill) only affects the current state.
func (g GraphicsState) Clone() GraphicsState {
	// Shallow copy - copies all fields including pointers
	clone := g

	// Note: Fill, Stroke, and ClipPath are pointer fields.
	// We intentionally do NOT deep copy them here.
	// This allows Push/Pop to save/restore the pointer values themselves.
	//
	// Example:
	//   surface.SetFill(fill1)      // currentState.Fill = &fill1
	//   surface.PushOpacity(0.5)    // saves &fill1 to stack
	//   surface.SetFill(fill2)      // currentState.Fill = &fill2
	//   surface.Pop()               // restores &fill1 from stack

	return clone
}
