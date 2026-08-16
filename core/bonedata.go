package spine

// Inherit determines how a bone inherits world transforms from parent bones.
type Inherit int

const (
	InheritNormal Inherit = iota
	InheritOnlyTranslation
	InheritNoRotationOrReflection
	InheritNoScale
	InheritNoScaleOrReflection
)

// BoneData stores the setup pose for a Bone.
type BoneData struct {
	Index  int
	Name   string
	Parent *BoneData
	Length float32

	X, Y, Rotation, ScaleX, ScaleY, ShearX, ShearY float32

	Inherit      Inherit
	SkinRequired bool

	// Nonessential.
	Color   Color
	Icon    string
	Visible bool
}

// NewBoneData creates bone setup pose data. parent may be nil for the root bone.
func NewBoneData(index int, name string, parent *BoneData) *BoneData {
	if index < 0 {
		panic("spine: index must be >= 0")
	}
	if name == "" {
		panic("spine: name cannot be empty")
	}
	return &BoneData{
		Index:  index,
		Name:   name,
		Parent: parent,
		ScaleX: 1,
		ScaleY: 1,
		Color:  Color{0.61, 0.61, 0.61, 1}, // 9b9b9bff
	}
}

func (b *BoneData) String() string { return b.Name }
