package spine

// IkConstraintData stores the setup pose for an IkConstraint.
type IkConstraintData struct {
	ConstraintDataBase

	// Bones that are constrained by this IK constraint.
	Bones []*BoneData

	// Target is the bone that is the IK target.
	Target *BoneData

	// BendDirection controls, for two bone IK, the bend direction of the IK
	// bones, either 1 or -1.
	BendDirection int

	// Compress: for one bone IK, when true and the target is too close, the
	// bone is scaled to reach it.
	Compress bool

	// Stretch: when true and the target is out of range, the parent bone is
	// scaled to reach it. For two bone IK: 1) the child bone's local Y
	// translation is set to 0, 2) stretch is not applied if Softness is > 0,
	// and 3) if the parent bone has local nonuniform scale, stretch is not
	// applied.
	Stretch bool

	// Uniform: when true and Compress or Stretch is used, the bone is scaled
	// on both the X and Y axes.
	Uniform bool

	// Mix is a percentage (0-1) that controls the mix between the constrained
	// and unconstrained rotation. For two bone IK: if the parent bone has
	// local nonuniform scale, the child bone's local Y translation is set to 0.
	Mix float32

	// Softness: for two bone IK, the target bone's distance from the maximum
	// reach of the bones where rotation begins to slow. The bones will not
	// straighten completely until the target is this far out of range.
	Softness float32
}

// NewIkConstraintData creates IK constraint setup pose data.
func NewIkConstraintData(name string) *IkConstraintData {
	if name == "" {
		panic("spine: name cannot be empty")
	}
	return &IkConstraintData{ConstraintDataBase: ConstraintDataBase{Name: name}}
}
