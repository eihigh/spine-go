package spine

// BlendMode determines how images are blended with existing pixels when drawn.
// The renderer maps these to backend-specific blend factors.
type BlendMode int

const (
	BlendModeNormal BlendMode = iota
	BlendModeAdditive
	BlendModeMultiply
	BlendModeScreen
)

// SlotData stores the setup pose for a Slot.
type SlotData struct {
	Index    int
	Name     string
	BoneData *BoneData

	// Color is used to tint the slot's attachment. If DarkColor is set, this
	// is the light color for two color tinting.
	Color Color

	// DarkColor is the dark color for two color tinting, or nil if two color
	// tinting is not used. The dark color's alpha is not used.
	DarkColor *Color

	// AttachmentName is the name of the attachment visible in the setup pose,
	// or empty if no attachment is visible.
	AttachmentName string

	BlendMode BlendMode

	// Nonessential.
	Visible bool
}

// NewSlotData creates slot setup pose data.
func NewSlotData(index int, name string, boneData *BoneData) *SlotData {
	if index < 0 {
		panic("spine: index must be >= 0")
	}
	if name == "" {
		panic("spine: name cannot be empty")
	}
	if boneData == nil {
		panic("spine: boneData cannot be nil")
	}
	return &SlotData{
		Index:    index,
		Name:     name,
		BoneData: boneData,
		Color:    Color{1, 1, 1, 1},
		Visible:  true,
	}
}

func (s *SlotData) String() string { return s.Name }
