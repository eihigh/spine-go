package spine

// TransformConstraintData stores the setup pose for a TransformConstraint.
type TransformConstraintData struct {
	ConstraintDataBase

	// Bones that will be modified by this transform constraint.
	Bones []*BoneData

	// Target is the bone whose world transform will be copied to the
	// constrained bones.
	Target *BoneData

	// Mixes are percentages (0-1) that control the mix between the constrained
	// and unconstrained values.
	MixRotate, MixX, MixY, MixScaleX, MixScaleY, MixShearY float32

	// Offsets added to the constrained bone values.
	OffsetRotation, OffsetX, OffsetY, OffsetScaleX, OffsetScaleY, OffsetShearY float32

	Relative, Local bool
}

// NewTransformConstraintData creates transform constraint setup pose data.
func NewTransformConstraintData(name string) *TransformConstraintData {
	if name == "" {
		panic("spine: name cannot be empty")
	}
	return &TransformConstraintData{ConstraintDataBase: ConstraintDataBase{Name: name}}
}
