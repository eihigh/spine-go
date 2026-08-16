package spine

// ConstraintDataBase stores fields common to all constraint data types.
type ConstraintDataBase struct {
	// Name is unique across all constraints in the skeleton of the same type.
	Name string

	// Order is the ordinal of this constraint for the order a skeleton's
	// constraints will be applied by Skeleton.UpdateWorldTransform.
	Order int

	// SkinRequired, when true, means Skeleton.UpdateWorldTransform only
	// updates this constraint if the skeleton's skin contains it.
	SkinRequired bool
}

// ConstraintData returns the common constraint data fields.
func (c *ConstraintDataBase) ConstraintData() *ConstraintDataBase { return c }

func (c *ConstraintDataBase) String() string { return c.Name }

// ConstraintData is implemented by IkConstraintData, TransformConstraintData,
// PathConstraintData and PhysicsConstraintData.
type ConstraintData interface {
	ConstraintData() *ConstraintDataBase
}

// SkinEntry stores an entry in the skin consisting of the slot index, the
// attachment name and the attachment.
type SkinEntry struct {
	SlotIndex int
	// Name is the name the attachment is associated with, equivalent to the
	// skin placeholder name in the Spine editor.
	Name       string
	Attachment Attachment
}

type skinKey struct {
	slotIndex int
	name      string
}

// Skin stores attachments by slot index and attachment name.
type Skin struct {
	// Name is unique across all skins in the skeleton.
	Name string

	entries []SkinEntry
	lookup  map[skinKey]int

	Bones       []*BoneData
	Constraints []ConstraintData

	// Nonessential.
	Color Color
}

// NewSkin creates a skin.
func NewSkin(name string) *Skin {
	if name == "" {
		panic("spine: name cannot be empty")
	}
	return &Skin{
		Name:   name,
		lookup: make(map[skinKey]int),
		Color:  Color{0.99607843, 0.61960787, 0.30980393, 1}, // fe9e4fff
	}
}

// SetAttachment adds an attachment to the skin for the specified slot index and name.
func (s *Skin) SetAttachment(slotIndex int, name string, attachment Attachment) {
	if attachment == nil {
		panic("spine: attachment cannot be nil")
	}
	key := skinKey{slotIndex, name}
	if i, ok := s.lookup[key]; ok {
		s.entries[i].Attachment = attachment
		return
	}
	s.lookup[key] = len(s.entries)
	s.entries = append(s.entries, SkinEntry{slotIndex, name, attachment})
}

// AddSkin adds all attachments, bones, and constraints from the specified skin
// to this skin.
func (s *Skin) AddSkin(skin *Skin) {
	if skin == nil {
		panic("spine: skin cannot be nil")
	}
	for _, data := range skin.Bones {
		if !containsBoneData(s.Bones, data) {
			s.Bones = append(s.Bones, data)
		}
	}
	for _, data := range skin.Constraints {
		if !containsConstraintData(s.Constraints, data) {
			s.Constraints = append(s.Constraints, data)
		}
	}
	for _, entry := range skin.entries {
		s.SetAttachment(entry.SlotIndex, entry.Name, entry.Attachment)
	}
}

// CopySkin adds all bones and constraints and copies of all attachments from
// the specified skin to this skin. Mesh attachments are not copied, instead a
// new linked mesh is created. The attachment copies can be modified without
// affecting the originals.
func (s *Skin) CopySkin(skin *Skin) {
	if skin == nil {
		panic("spine: skin cannot be nil")
	}
	for _, data := range skin.Bones {
		if !containsBoneData(s.Bones, data) {
			s.Bones = append(s.Bones, data)
		}
	}
	for _, data := range skin.Constraints {
		if !containsConstraintData(s.Constraints, data) {
			s.Constraints = append(s.Constraints, data)
		}
	}
	for _, entry := range skin.entries {
		if mesh, ok := entry.Attachment.(*MeshAttachment); ok {
			s.SetAttachment(entry.SlotIndex, entry.Name, mesh.NewLinkedMesh())
		} else if entry.Attachment != nil {
			s.SetAttachment(entry.SlotIndex, entry.Name, entry.Attachment.Copy())
		}
	}
}

// Attachment returns the attachment for the specified slot index and name, or nil.
func (s *Skin) Attachment(slotIndex int, name string) Attachment {
	if i, ok := s.lookup[skinKey{slotIndex, name}]; ok {
		return s.entries[i].Attachment
	}
	return nil
}

// RemoveAttachment removes the attachment in the skin for the specified slot
// index and name, if any.
func (s *Skin) RemoveAttachment(slotIndex int, name string) {
	key := skinKey{slotIndex, name}
	i, ok := s.lookup[key]
	if !ok {
		return
	}
	delete(s.lookup, key)
	s.entries = append(s.entries[:i], s.entries[i+1:]...)
	for j := i; j < len(s.entries); j++ {
		s.lookup[skinKey{s.entries[j].SlotIndex, s.entries[j].Name}] = j
	}
}

// Attachments returns all attachments in this skin.
func (s *Skin) Attachments() []SkinEntry { return s.entries }

// AttachmentsForSlot appends all attachments in this skin for the specified
// slot index to dst and returns it.
func (s *Skin) AttachmentsForSlot(dst []SkinEntry, slotIndex int) []SkinEntry {
	if slotIndex < 0 {
		panic("spine: slotIndex must be >= 0")
	}
	for _, entry := range s.entries {
		if entry.SlotIndex == slotIndex {
			dst = append(dst, entry)
		}
	}
	return dst
}

// Clear clears all attachments, bones, and constraints.
func (s *Skin) Clear() {
	s.entries = s.entries[:0]
	s.lookup = make(map[skinKey]int)
	s.Bones = s.Bones[:0]
	s.Constraints = s.Constraints[:0]
}

func (s *Skin) String() string { return s.Name }

// AttachAll attaches each attachment in this skin if the corresponding
// attachment in the old skin is currently attached.
func (s *Skin) AttachAll(skeleton *Skeleton, oldSkin *Skin) {
	for _, entry := range oldSkin.entries {
		slotIndex := entry.SlotIndex
		slot := skeleton.Slots[slotIndex]
		if slot.attachment == entry.Attachment {
			attachment := s.Attachment(slotIndex, entry.Name)
			if attachment != nil {
				slot.SetAttachment(attachment)
			}
		}
	}
}

func containsBoneData(list []*BoneData, data *BoneData) bool {
	for _, d := range list {
		if d == data {
			return true
		}
	}
	return false
}

func containsConstraintData(list []ConstraintData, data ConstraintData) bool {
	for _, d := range list {
		if d == data {
			return true
		}
	}
	return false
}
