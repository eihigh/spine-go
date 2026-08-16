package spine

import "math"

// Math helpers mirroring spine-libgdx's SpineUtils. All angles produced and
// consumed by the runtime are float32, matching the reference implementation
// so numeric comparisons against other runtimes hold.
const (
	piF     = float32(3.1415927)
	pi2     = piF * 2
	invPI2  = 1 / pi2
	radDeg  = 180 / piF
	degRad  = piF / 180
	maxF32  = float32(math.MaxFloat32)
	epsilon = 0.00001
)

func cos(radians float32) float32    { return float32(math.Cos(float64(radians))) }
func sin(radians float32) float32    { return float32(math.Sin(float64(radians))) }
func cosDeg(degrees float32) float32 { return float32(math.Cos(float64(degrees * degRad))) }
func sinDeg(degrees float32) float32 { return float32(math.Sin(float64(degrees * degRad))) }

func atan2(y, x float32) float32    { return float32(math.Atan2(float64(y), float64(x))) }
func atan2Deg(y, x float32) float32 { return atan2(y, x) * radDeg }

func sqrt(x float32) float32 { return float32(math.Sqrt(float64(x))) }

// mod32 is Java's % operator for floats (truncated remainder).
func mod32(x, y float32) float32 { return float32(math.Mod(float64(x), float64(y))) }
func pow(x, y float32) float32   { return float32(math.Pow(float64(x), float64(y))) }

func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func maxF(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func minF(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func clamp(value, min, max float32) float32 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func signum(x float32) float32 {
	if x > 0 {
		return 1
	}
	if x < 0 {
		return -1
	}
	return 0
}

func isNaN32(x float32) bool { return x != x }

func ceilInt(x float32) int  { return int(math.Ceil(float64(x))) }
func floorInt(x float32) int { return int(math.Floor(float64(x))) }
