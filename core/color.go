package spine

import (
	"fmt"
	"strconv"
)

// Color stores red, green, blue and alpha components in the range [0,1].
// It mirrors libgdx's Color as used by the Spine runtime.
type Color struct {
	R, G, B, A float32
}

// NewColorRGBA returns a color with the given components.
func NewColorRGBA(r, g, b, a float32) Color { return Color{r, g, b, a} }

// Set sets the components and clamps them to [0,1].
func (c *Color) Set(r, g, b, a float32) {
	c.R = r
	c.G = g
	c.B = b
	c.A = a
	c.Clamp()
}

// SetColor sets this color from another color.
func (c *Color) SetColor(o Color) { *c = o }

// Add adds the given components and clamps the result.
func (c *Color) Add(r, g, b, a float32) {
	c.R += r
	c.G += g
	c.B += b
	c.A += a
	c.Clamp()
}

// Clamp clamps all components to [0,1].
func (c *Color) Clamp() {
	c.R = clamp(c.R, 0, 1)
	c.G = clamp(c.G, 0, 1)
	c.B = clamp(c.B, 0, 1)
	c.A = clamp(c.A, 0, 1)
}

// ColorFromString parses a hex color string "rrggbb" or "rrggbbaa"
// (with optional leading '#'), as found in Spine JSON exports.
func ColorFromString(hex string) (Color, error) {
	if len(hex) > 0 && hex[0] == '#' {
		hex = hex[1:]
	}
	if len(hex) != 6 && len(hex) != 8 {
		return Color{}, fmt.Errorf("spine: invalid color: %q", hex)
	}
	v, err := strconv.ParseUint(hex, 16, 64)
	if err != nil {
		return Color{}, fmt.Errorf("spine: invalid color: %q", hex)
	}
	c := Color{A: 1}
	if len(hex) == 8 {
		c.A = float32(v&0xff) / 255
		v >>= 8
	}
	c.B = float32(v&0xff) / 255
	c.G = float32((v>>8)&0xff) / 255
	c.R = float32((v>>16)&0xff) / 255
	return c, nil
}
