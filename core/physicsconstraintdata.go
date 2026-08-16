package spine

// PhysicsConstraintData stores the setup pose for a PhysicsConstraint.
type PhysicsConstraintData struct {
	ConstraintDataBase

	// Bone constrained by this physics constraint.
	Bone *BoneData

	X, Y, Rotate, ScaleX, ShearX, Limit                  float32
	Step, Inertia, Strength, Damping, MassInverse        float32
	Wind, Gravity, Mix                                   float32
	InertiaGlobal, StrengthGlobal, DampingGlobal         bool
	MassGlobal, WindGlobal, GravityGlobal, MixGlobal     bool
}

// NewPhysicsConstraintData creates physics constraint setup pose data.
func NewPhysicsConstraintData(name string) *PhysicsConstraintData {
	if name == "" {
		panic("spine: name cannot be empty")
	}
	return &PhysicsConstraintData{ConstraintDataBase: ConstraintDataBase{Name: name}}
}
