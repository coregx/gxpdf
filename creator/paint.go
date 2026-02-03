package creator

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Paint represents a paint source for fill and stroke operations.
//
// Paint can be a solid color, a gradient, or a pattern (future).
// This interface allows uniform handling of different paint types.
//
// Implemented by:
//   - Color (RGB solid color)
//   - ColorRGBA (RGB with alpha)
//   - ColorCMYK (CMYK solid color)
//   - Gradient (linear/radial gradient)
//
// Example:
//
//	var paint Paint
//	paint = Red                    // Solid color
//	paint = NewColorRGBA(1, 0, 0, 0.5) // Transparent red
//	paint = NewLinearGradient(0, 0, 100, 0) // Gradient
type Paint interface {
	// isPaint is a marker method to restrict implementations.
	isPaint()
}

// Ensure types implement Paint interface.
var (
	_ Paint = Color{}
	_ Paint = ColorRGBA{}
	_ Paint = ColorCMYK{}
	_ Paint = (*Gradient)(nil)
)

// isPaint implementations for each paint type.
func (Color) isPaint()     {}
func (ColorRGBA) isPaint() {}
func (ColorCMYK) isPaint() {}
func (*Gradient) isPaint() {}

// RGB creates a Color from 8-bit RGB values (0-255).
//
// This is a convenience function for creating colors from common 8-bit
// color values instead of normalized floats.
//
// Parameters:
//   - r: Red component (0 to 255)
//   - g: Green component (0 to 255)
//   - b: Blue component (0 to 255)
//
// Example:
//
//	red := creator.RGB(255, 0, 0)
//	green := creator.RGB(0, 255, 0)
//	gray := creator.RGB(128, 128, 128)
func RGB(r, g, b uint8) Color {
	return Color{
		R: float64(r) / 255.0,
		G: float64(g) / 255.0,
		B: float64(b) / 255.0,
	}
}

// RGBA creates a ColorRGBA from 8-bit RGB values and normalized alpha.
//
// Parameters:
//   - r: Red component (0 to 255)
//   - g: Green component (0 to 255)
//   - b: Blue component (0 to 255)
//   - a: Alpha component (0.0 = transparent, 1.0 = opaque)
//
// Example:
//
//	transparentRed := creator.RGBA(255, 0, 0, 0.5)
//	semiTransparentBlue := creator.RGBA(0, 0, 255, 0.3)
func RGBA(r, g, b uint8, a float64) ColorRGBA {
	return ColorRGBA{
		R: float64(r) / 255.0,
		G: float64(g) / 255.0,
		B: float64(b) / 255.0,
		A: a,
	}
}

// Hex creates a Color from a hex color string.
//
// Supported formats:
//   - "#RGB" (short form, e.g., "#F00" = red)
//   - "#RRGGBB" (long form, e.g., "#FF0000" = red)
//   - "RGB" (without #)
//   - "RRGGBB" (without #)
//
// Parameters:
//   - hex: Hex color string
//
// Example:
//
//	red, _ := creator.Hex("#FF0000")
//	green, _ := creator.Hex("00FF00")
//	blue, _ := creator.Hex("#00F")
func Hex(hex string) (Color, error) {
	// Remove leading # if present
	hex = strings.TrimPrefix(hex, "#")

	var r, g, b uint8

	switch len(hex) {
	case 3:
		// Short form: #RGB -> #RRGGBB
		rv, err := strconv.ParseUint(string(hex[0]), 16, 8)
		if err != nil {
			return Color{}, fmt.Errorf("invalid hex color: %w", err)
		}
		gv, err := strconv.ParseUint(string(hex[1]), 16, 8)
		if err != nil {
			return Color{}, fmt.Errorf("invalid hex color: %w", err)
		}
		bv, err := strconv.ParseUint(string(hex[2]), 16, 8)
		if err != nil {
			return Color{}, fmt.Errorf("invalid hex color: %w", err)
		}
		r = uint8(rv*16 + rv)
		g = uint8(gv*16 + gv)
		b = uint8(bv*16 + bv)

	case 6:
		// Long form: #RRGGBB
		rv, err := strconv.ParseUint(hex[0:2], 16, 8)
		if err != nil {
			return Color{}, fmt.Errorf("invalid hex color: %w", err)
		}
		gv, err := strconv.ParseUint(hex[2:4], 16, 8)
		if err != nil {
			return Color{}, fmt.Errorf("invalid hex color: %w", err)
		}
		bv, err := strconv.ParseUint(hex[4:6], 16, 8)
		if err != nil {
			return Color{}, fmt.Errorf("invalid hex color: %w", err)
		}
		r = uint8(rv)
		g = uint8(gv)
		b = uint8(bv)

	default:
		return Color{}, fmt.Errorf("invalid hex color length: expected 3 or 6 characters, got %d", len(hex))
	}

	return RGB(r, g, b), nil
}

// GrayN creates a Color from a grayscale value (0-255).
//
// This is a convenience function for creating gray colors from numeric values.
// All RGB components are set to the same value.
//
// Note: Use the Gray variable for the predefined 50% gray color.
//
// Parameters:
//   - value: Grayscale value (0 = black, 255 = white)
//
// Example:
//
//	black := creator.GrayN(0)
//	gray := creator.GrayN(128)
//	white := creator.GrayN(255)
func GrayN(value uint8) Color {
	return RGB(value, value, value)
}

// validateColorRGBA validates that color components are in valid range.
func validateColorRGBA(c ColorRGBA) error {
	if err := validateColor(Color{R: c.R, G: c.G, B: c.B}); err != nil {
		return err
	}
	if c.A < 0 || c.A > 1 {
		return fmt.Errorf("alpha component out of range [0, 1]: %f", c.A)
	}
	return nil
}

// validateColorCMYK validates that color components are in valid range.
func validateColorCMYK(c ColorCMYK) error {
	if c.C < 0 || c.C > 1 {
		return fmt.Errorf("cyan component out of range [0, 1]: %f", c.C)
	}
	if c.M < 0 || c.M > 1 {
		return fmt.Errorf("magenta component out of range [0, 1]: %f", c.M)
	}
	if c.Y < 0 || c.Y > 1 {
		return fmt.Errorf("yellow component out of range [0, 1]: %f", c.Y)
	}
	if c.K < 0 || c.K > 1 {
		return fmt.Errorf("black component out of range [0, 1]: %f", c.K)
	}
	return nil
}

// Errors
var (
	// ErrInvalidColor is returned when a color has invalid component values.
	ErrInvalidColor = errors.New("invalid color component values")
)
