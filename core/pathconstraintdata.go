package spine

// PositionMode controls how the first bone is positioned along the path.
type PositionMode int

const (
	PositionModeFixed PositionMode = iota
	PositionModePercent
)

// SpacingMode controls how bones after the first bone are positioned along the path.
type SpacingMode int

const (
	SpacingModeLength SpacingMode = iota
	SpacingModeFixed
	SpacingModePercent
	SpacingModeProportional
)

// RotateMode controls how bones are rotated, translated, and scaled to match the path.
type RotateMode int

const (
	RotateModeTangent RotateMode = iota
	RotateModeChain
	// RotateModeChainScale: when chain scale, constrained bones should all
	// have the same parent. That way when the path constraint scales a bone,
	// it doesn't affect other constrained bones.
	RotateModeChainScale
)

// PathConstraintData stores the setup pose for a PathConstraint.
type PathConstraintData struct {
	ConstraintDataBase

	// Bones that will be modified by this path constraint.
	Bones []*BoneData

	// Target is the slot whose path attachment will be used to constrain the bones.
	Target *SlotData

	// PositionMode is the mode for positioning the first bone on the path.
	PositionMode PositionMode

	// SpacingMode is the mode for positioning the bones after the first bone.
	SpacingMode SpacingMode

	// RotateMode is the mode for adjusting the rotation of the bones.
	RotateMode RotateMode

	// OffsetRotation is an offset added to the constrained bone rotation.
	OffsetRotation float32

	// Position is the position along the path.
	Position float32

	// Spacing is the spacing between bones.
	Spacing float32

	// MixRotate, MixX, MixY are percentages (0-1) that control the mix between
	// the constrained and unconstrained values.
	MixRotate, MixX, MixY float32
}

// NewPathConstraintData creates path constraint setup pose data.
func NewPathConstraintData(name string) *PathConstraintData {
	if name == "" {
		panic("spine: name cannot be empty")
	}
	return &PathConstraintData{ConstraintDataBase: ConstraintDataBase{Name: name}}
}
