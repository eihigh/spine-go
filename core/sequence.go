package spine

import (
	"strconv"
	"sync/atomic"
)

// SequenceMode determines how a Sequence is played by a SequenceTimeline.
type SequenceMode int

const (
	SequenceModeHold SequenceMode = iota
	SequenceModeOnce
	SequenceModeLoop
	SequenceModePingpong
	SequenceModeOnceReverse
	SequenceModeLoopReverse
	SequenceModePingpongReverse
)

var nextSequenceID int64

// Sequence stores frame-by-frame texture regions for an attachment.
type Sequence struct {
	id int

	// Regions holds one texture region per frame.
	Regions []*TextureRegion

	// Start is the index of the first frame in the file names.
	Start int

	// Digits is the number of digits used for frame numbers in the file names.
	Digits int

	// SetupIndex is the index of the region to show for the setup pose.
	SetupIndex int
}

// NewSequence creates a sequence with the given frame count.
func NewSequence(count int) *Sequence {
	return &Sequence{
		id:      int(atomic.AddInt64(&nextSequenceID, 1) - 1),
		Regions: make([]*TextureRegion, count),
	}
}

// ID returns a unique ID for this sequence.
func (s *Sequence) ID() int { return s.id }

// Copy returns a copy of the sequence sharing the same regions.
func (s *Sequence) Copy() *Sequence {
	c := NewSequence(len(s.Regions))
	copy(c.Regions, s.Regions)
	c.Start = s.Start
	c.Digits = s.Digits
	c.SetupIndex = s.SetupIndex
	return c
}

// Apply sets the attachment's region to the one indicated by the slot's
// SequenceIndex, updating the attachment's UVs if the region changed.
func (s *Sequence) Apply(slot *Slot, attachment HasTextureRegion) {
	index := slot.SequenceIndex
	if index == -1 {
		index = s.SetupIndex
	}
	if index >= len(s.Regions) {
		index = len(s.Regions) - 1
	}
	region := s.Regions[index]
	if attachment.Region() != region {
		attachment.SetRegion(region)
		attachment.UpdateRegion()
	}
}

// Path returns the file path for the given frame index.
func (s *Sequence) Path(basePath string, index int) string {
	frame := strconv.Itoa(s.Start + index)
	buf := make([]byte, 0, len(basePath)+s.Digits)
	buf = append(buf, basePath...)
	for i := s.Digits - len(frame); i > 0; i-- {
		buf = append(buf, '0')
	}
	buf = append(buf, frame...)
	return string(buf)
}
