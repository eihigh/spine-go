package spine

// AnimationStateData stores mix (crossfade) durations to be applied when
// AnimationState animations are changed.
type AnimationStateData struct {
	// SkeletonData is used to look up animations when they are specified by
	// name.
	SkeletonData *SkeletonData

	// DefaultMix is the mix duration to use when no mix duration has been
	// defined between two animations.
	DefaultMix float32

	animationToMixTime map[animationPair]float32
}

type animationPair struct {
	from, to *Animation
}

// NewAnimationStateData creates animation state data.
func NewAnimationStateData(skeletonData *SkeletonData) *AnimationStateData {
	if skeletonData == nil {
		panic("spine: skeletonData cannot be nil")
	}
	return &AnimationStateData{
		SkeletonData:       skeletonData,
		animationToMixTime: make(map[animationPair]float32),
	}
}

// SetMixByName sets a mix duration by animation name.
//
// See SetMix.
func (d *AnimationStateData) SetMixByName(fromName, toName string, duration float32) {
	from := d.SkeletonData.FindAnimation(fromName)
	if from == nil {
		panic("spine: animation not found: " + fromName)
	}
	to := d.SkeletonData.FindAnimation(toName)
	if to == nil {
		panic("spine: animation not found: " + toName)
	}
	d.SetMix(from, to, duration)
}

// SetMix sets the mix duration when changing from the specified animation to
// the other.
//
// See TrackEntry.MixDuration.
func (d *AnimationStateData) SetMix(from, to *Animation, duration float32) {
	if from == nil {
		panic("spine: from cannot be nil")
	}
	if to == nil {
		panic("spine: to cannot be nil")
	}
	d.animationToMixTime[animationPair{from, to}] = duration
}

// GetMix returns the mix duration to use when changing from the specified
// animation to the other, or the DefaultMix if no mix duration has been set.
func (d *AnimationStateData) GetMix(from, to *Animation) float32 {
	if from == nil {
		panic("spine: from cannot be nil")
	}
	if to == nil {
		panic("spine: to cannot be nil")
	}
	if duration, ok := d.animationToMixTime[animationPair{from, to}]; ok {
		return duration
	}
	return d.DefaultMix
}
