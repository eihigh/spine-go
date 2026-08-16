package spine

// Slot stores a slot's current pose. Slots organize attachments for
// Skeleton.DrawOrder purposes and provide a place to store state for an
// attachment. State cannot be stored in an attachment itself because
// attachments are stateless and may be shared across multiple skeletons.
type Slot struct {
	Data *SlotData
	Bone *Bone

	// Color is used to tint the slot's attachment. If DarkColor is set, this
	// is the light color for two color tinting.
	Color Color

	// DarkColor is the dark color for two color tinting, or nil if two color
	// tinting is not used. The dark color's alpha is not used.
	DarkColor *Color

	attachment Attachment

	// SequenceIndex is the index of the texture region to display when the
	// slot's attachment has a Sequence. -1 represents the sequence's setup index.
	SequenceIndex int

	// Deform holds values to deform the slot's attachment. For an unweighted
	// mesh, the entries are local positions for each vertex. For a weighted
	// mesh, the entries are an offset for each vertex which will be added to
	// the mesh's local vertex positions.
	Deform []float32

	attachmentState int
}

// NewSlot creates a slot.
func NewSlot(data *SlotData, bone *Bone) *Slot {
	if data == nil {
		panic("spine: data cannot be nil")
	}
	if bone == nil {
		panic("spine: bone cannot be nil")
	}
	s := &Slot{Data: data, Bone: bone}
	if data.DarkColor != nil {
		s.DarkColor = &Color{}
	}
	s.SetToSetupPose()
	return s
}

// Skeleton returns the skeleton this slot belongs to.
func (s *Slot) Skeleton() *Skeleton { return s.Bone.Skeleton }

// Attachment returns the current attachment for the slot, or nil if the slot
// has no attachment.
func (s *Slot) Attachment() Attachment { return s.attachment }

// SetAttachment sets the slot's attachment and, if the attachment changed,
// resets SequenceIndex and clears the Deform. The deform is not cleared if the
// old attachment has the same timeline attachment as the specified attachment.
func (s *Slot) SetAttachment(attachment Attachment) {
	if s.attachment == attachment {
		return
	}
	va := AsVertexAttachment(attachment)
	cva := AsVertexAttachment(s.attachment)
	if va == nil || cva == nil || va.TimelineAttachment != cva.TimelineAttachment {
		s.Deform = s.Deform[:0]
	}
	s.attachment = attachment
	s.SequenceIndex = -1
}

// SetToSetupPose sets this slot to the setup pose.
func (s *Slot) SetToSetupPose() {
	s.Color = s.Data.Color
	if s.DarkColor != nil {
		*s.DarkColor = *s.Data.DarkColor
	}
	if s.Data.AttachmentName == "" {
		s.SetAttachment(nil)
	} else {
		s.attachment = nil
		s.SetAttachment(s.Bone.Skeleton.AttachmentByIndex(s.Data.Index, s.Data.AttachmentName))
	}
}

func (s *Slot) String() string { return s.Data.Name }
